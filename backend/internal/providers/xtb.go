package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/shopspring/decimal"
	"github.com/undeschimb/undeschimb/internal/domain"
)

const xtbBaseURL = "https://www.xtb.com/ro/forex/"

type XTBProvider struct {
	client        *http.Client
	extraHolidays map[string]struct{}
}

func NewXTBProvider(client *http.Client, extraHolidays map[string]struct{}) *XTBProvider {
	return &XTBProvider{client: client, extraHolidays: extraHolidays}
}

func (p *XTBProvider) ID() string { return "xtb" }

type bidAsk struct {
	Bid decimal.Decimal
	Ask decimal.Decimal
	URL string
}

type xtbWidgetConnection struct {
	URL        string
	Endpoint   string
	User       string
	AccountID  string
	AccessCode string
}

type xtbPairMeta struct {
	Connection xtbWidgetConnection
	Key        string
	URL        string
}

func (p *XTBProvider) Fetch(ctx context.Context) ([]domain.RateSnapshot, error) {
	now := time.Now().In(bucharestLocation())
	fee := xtbFee(now, p.extraHolidays)
	quotes := make([]domain.RateSnapshot, 0, 4)
	for _, currency := range []string{"EUR", "USD", "GBP", "CHF"} {
		quote, err := p.ronPair(ctx, currency)
		if err != nil {
			return nil, fmt.Errorf("read XTB %s/RON quote: %w", currency, err)
		}
		// XTB charges the conversion fee against the converted amount. Persist
		// an effective rate, allowing the common calculator to compare it with banks.
		quotes = append(quotes, domain.RateSnapshot{
			Provider: "xtb", Currency: currency,
			BuyRate:    quote.Bid.Mul(decimal.NewFromInt(1).Sub(fee)),
			SellRate:   quote.Ask.Div(decimal.NewFromInt(1).Sub(fee)),
			FeePercent: fee.Mul(decimal.NewFromInt(100)),
			SourceURL:  quote.URL, EffectiveAt: now.UTC(), FetchedAt: now.UTC(),
		})
	}
	return quotes, nil
}

func (p *XTBProvider) ronPair(ctx context.Context, currency string) (bidAsk, error) {
	direct := strings.ToLower(currency) + "-ron"
	if quote, err := p.fetchPair(ctx, direct); err == nil {
		return quote, nil
	}

	usdRon, err := p.fetchPair(ctx, "usd-ron")
	if err != nil {
		return bidAsk{}, err
	}
	if currency == "USD" {
		return usdRon, nil
	}
	if currency == "CHF" {
		usdChf, err := p.fetchPair(ctx, "usd-chf")
		if err != nil {
			return bidAsk{}, err
		}
		return bidAsk{
			Bid: decimal.NewFromInt(1).Div(usdChf.Ask).Mul(usdRon.Bid),
			Ask: decimal.NewFromInt(1).Div(usdChf.Bid).Mul(usdRon.Ask),
			URL: xtbBaseURL,
		}, nil
	}
	cross, err := p.fetchPair(ctx, strings.ToLower(currency)+"-usd")
	if err != nil {
		return bidAsk{}, err
	}
	return bidAsk{Bid: cross.Bid.Mul(usdRon.Bid), Ask: cross.Ask.Mul(usdRon.Ask), URL: xtbBaseURL}, nil
}

func (p *XTBProvider) fetchPair(ctx context.Context, slug string) (bidAsk, error) {
	url := xtbBaseURL + slug
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return bidAsk{}, err
	}
	request.Header.Set("Accept-Language", "ro-RO,ro;q=0.9")
	request.Header.Set("User-Agent", "UndeSchimb/1.0 (+https://github.com/undeschimb/undeschimb)")
	response, err := p.client.Do(request)
	if err != nil {
		return bidAsk{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return bidAsk{}, fmt.Errorf("quote page returned status %d", response.StatusCode)
	}
	payload, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return bidAsk{}, err
	}
	meta, err := ExtractXTBPairMeta(string(payload), url)
	if err != nil {
		return bidAsk{}, err
	}
	bid, ask, err := fetchXTBWidgetQuote(ctx, meta)
	if err != nil {
		return bidAsk{}, err
	}
	return bidAsk{Bid: bid, Ask: ask, URL: url}, nil
}

func ExtractXTBPairMeta(document, sourceURL string) (xtbPairMeta, error) {
	blockPattern := regexp.MustCompile(`(?s)window\.websocketClientData\s*=\s*\{(.*?)\};`)
	blockMatch := blockPattern.FindStringSubmatch(document)
	if len(blockMatch) != 2 {
		return xtbPairMeta{}, fmt.Errorf("XTB widget connection configuration is missing")
	}
	field := func(name string) string {
		pattern := regexp.MustCompile(`(?m)["']` + regexp.QuoteMeta(name) + `["']\s*:\s*["']([^"']+)["']`)
		match := pattern.FindStringSubmatch(blockMatch[1])
		if len(match) == 2 {
			return match[1]
		}
		return ""
	}
	connection := xtbWidgetConnection{
		URL: field("url"), Endpoint: field("endpoint"), User: field("user"), AccountID: field("accountId"), AccessCode: field("accessCode"),
	}
	symbolPattern := regexp.MustCompile(`(?m)["']symbolXapi5["']\s*:\s*["']([^"']+)["']`)
	symbolMatch := symbolPattern.FindStringSubmatch(document)
	if connection.URL == "" || connection.Endpoint == "" || connection.User == "" || connection.AccountID == "" || connection.AccessCode == "" || len(symbolMatch) != 2 {
		return xtbPairMeta{}, fmt.Errorf("XTB page does not expose a complete widget quote configuration")
	}
	return xtbPairMeta{Connection: connection, Key: symbolMatch[1], URL: sourceURL}, nil
}

type xtbSocketMessage struct {
	ReqID    string `json:"reqId"`
	Status   int    `json:"status"`
	Response []struct {
		Element struct {
			Elements []struct {
				Value struct {
					Tick xtbTick `json:"xcfdtick"`
				} `json:"value"`
			} `json:"elements"`
		} `json:"element"`
	} `json:"response"`
}

type xtbTick struct {
	Key string      `json:"key"`
	Bid json.Number `json:"bid"`
	Ask json.Number `json:"ask"`
}

func fetchXTBWidgetQuote(ctx context.Context, meta xtbPairMeta) (decimal.Decimal, decimal.Decimal, error) {
	connection, _, err := websocket.DefaultDialer.DialContext(ctx, meta.Connection.URL, nil)
	if err != nil {
		return decimal.Zero, decimal.Zero, fmt.Errorf("connect to XTB public widget: %w", err)
	}
	defer connection.Close()
	var requestDeadline time.Time
	if contextDeadline, ok := ctx.Deadline(); ok {
		requestDeadline = contextDeadline
	} else {
		requestDeadline = time.Now().Add(12 * time.Second)
	}
	_ = connection.SetReadDeadline(requestDeadline)
	_ = connection.SetWriteDeadline(requestDeadline)
	logon := map[string]any{"reqId": "logonRestricted", "command": map[string]any{"CoreAPI": map[string]any{
		"endpoint":        meta.Connection.Endpoint,
		"logonRestricted": map[string]string{"user": meta.Connection.User, "accessCode": meta.Connection.AccessCode},
	}}}
	if err := connection.WriteJSON(logon); err != nil {
		return decimal.Zero, decimal.Zero, err
	}
	for attempts := 0; attempts < 4; attempts++ {
		var message xtbSocketMessage
		if err := connection.ReadJSON(&message); err != nil {
			return decimal.Zero, decimal.Zero, err
		}
		if message.ReqID == "logonRestricted" {
			if message.Status != 0 {
				return decimal.Zero, decimal.Zero, fmt.Errorf("XTB public widget rejected the restricted logon")
			}
			quoteRequest := map[string]any{"reqId": "getAndSubscribeElement", "command": map[string]any{"CoreAPI": map[string]any{
				"endpoint": meta.Connection.Endpoint, "accountId": meta.Connection.AccountID,
				"getAndSubscribeElement": map[string]any{"eid": 2, "keys": []string{meta.Key}},
			}}}
			if err := connection.WriteJSON(quoteRequest); err != nil {
				return decimal.Zero, decimal.Zero, err
			}
			continue
		}
		if message.ReqID == "getAndSubscribeElement" {
			return quoteFromMessage(message, meta.Key)
		}
	}
	return decimal.Zero, decimal.Zero, fmt.Errorf("XTB widget did not return a quote")
}

func quoteFromMessage(message xtbSocketMessage, key string) (decimal.Decimal, decimal.Decimal, error) {
	for _, response := range message.Response {
		for _, element := range response.Element.Elements {
			tick := element.Value.Tick
			if tick.Key != "" && tick.Key != key {
				continue
			}
			bid, bidErr := decimal.NewFromString(tick.Bid.String())
			ask, askErr := decimal.NewFromString(tick.Ask.String())
			if bidErr != nil || askErr != nil || !bid.IsPositive() || !ask.IsPositive() || bid.GreaterThan(ask) {
				return decimal.Zero, decimal.Zero, fmt.Errorf("XTB returned an invalid bid/ask quote")
			}
			return bid, ask, nil
		}
	}
	return decimal.Zero, decimal.Zero, fmt.Errorf("XTB widget response did not contain the requested quote")
}

func xtbFee(now time.Time, extraHolidays map[string]struct{}) decimal.Decimal {
	if isRomanianMarketHoliday(now, extraHolidays) {
		return decimal.RequireFromString("0.008")
	}
	return decimal.RequireFromString("0.005")
}

func bucharestLocation() *time.Location {
	return domain.BucharestLocation()
}

func isRomanianMarketHoliday(now time.Time, extra map[string]struct{}) bool {
	return domain.IsRomanianNonWorkingDay(now, extra)
}
