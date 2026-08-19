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

	"github.com/shopspring/decimal"
	"github.com/undeschimb/undeschimb/internal/domain"
)

const (
	BCRSourceURL        = "https://www.bcr.ro/ro/curs-valutar"
	BCRRateAPIURL       = "https://api.bcr.ro/api/v1/bcr/external/getExchangeRates"
	RaiffeisenSourceURL = "https://www.raiffeisen.ro/ro/home/curs-valutar.html"
	INGSourceURL        = "https://ing.ro/persoane-fizice/curs-valutar"
	CECSourceURL        = "https://home.ceconline.ro/Info/ExchangeRates?language=ro-RO"
)

type BCRProvider struct{ client *http.Client }

func NewBCRProvider(client *http.Client) *BCRProvider { return &BCRProvider{client: client} }
func (p *BCRProvider) ID() string                     { return "bcr" }

func (p *BCRProvider) Fetch(ctx context.Context) ([]domain.RateSnapshot, error) {
	payload, err := fetchPublicPayload(ctx, p.client, BCRRateAPIURL, "BCR rate API")
	if err != nil {
		return nil, err
	}
	return ParseBCRRates(payload, time.Now().UTC())
}

func ParseBCRRates(payload []byte, fetchedAt time.Time) ([]domain.RateSnapshot, error) {
	var document struct {
		FX []struct {
			Currency     string `json:"currency"`
			ExchangeRate struct {
				LastModified string      `json:"lastModified"`
				Buy          json.Number `json:"buy"`
				Sell         json.Number `json:"sell"`
			} `json:"exchangeRate"`
		} `json:"fx"`
	}
	decoder := json.NewDecoder(strings.NewReader(string(payload)))
	decoder.UseNumber()
	if err := decoder.Decode(&document); err != nil {
		return nil, fmt.Errorf("decode BCR rates: %w", err)
	}
	snapshots := make([]domain.RateSnapshot, 0, 4)
	for _, rate := range document.FX {
		currency := domain.NormalizeCurrency(rate.Currency)
		if !domain.ValidCurrency(currency) || currency == "RON" {
			continue
		}
		buy, buyErr := decimal.NewFromString(rate.ExchangeRate.Buy.String())
		sell, sellErr := decimal.NewFromString(rate.ExchangeRate.Sell.String())
		if buyErr != nil || sellErr != nil || !validRetailQuote(buy, sell) {
			return nil, fmt.Errorf("invalid BCR %s quote", currency)
		}
		effectiveAt := fetchedAt
		if parsed, err := time.Parse(time.RFC3339, rate.ExchangeRate.LastModified); err == nil {
			effectiveAt = parsed
		}
		snapshots = append(snapshots, retailSnapshot("bcr", currency, buy, sell, BCRSourceURL, effectiveAt, fetchedAt))
	}
	return requireFourRetailSnapshots("BCR", snapshots)
}

type RaiffeisenProvider struct{ client *http.Client }

func NewRaiffeisenProvider(client *http.Client) *RaiffeisenProvider {
	return &RaiffeisenProvider{client: client}
}
func (p *RaiffeisenProvider) ID() string { return "raiffeisen" }

func (p *RaiffeisenProvider) Fetch(ctx context.Context) ([]domain.RateSnapshot, error) {
	date := time.Now().In(bucharestLocation()).Format("20060102")
	url := "https://www.raiffeisen.ro/ro.exchangerates." + date + ".BASE.EUR-USD-GBP-CAD-HUF-MDL-CHF-SEK-JPY-DKK-RUB-TRY-CZK-PLN-EGP.RON.TARGET.json"
	payload, err := fetchPublicPayload(ctx, p.client, url, "Raiffeisen account rate API")
	if err != nil {
		return nil, err
	}
	return ParseRaiffeisenRates(payload, time.Now().UTC())
}

func ParseRaiffeisenRates(payload []byte, fetchedAt time.Time) ([]domain.RateSnapshot, error) {
	var document struct {
		Rates []struct {
			CurrencyList []struct {
				CurrencyPair struct {
					Left struct {
						Code string `json:"code"`
					} `json:"left"`
				} `json:"currencyPair"`
				BuyRate struct {
					Value json.Number `json:"value"`
				} `json:"buyRate"`
				SellRate struct {
					Value json.Number `json:"value"`
				} `json:"sellRate"`
				CreatedAt string `json:"listCreationDateTime"`
			} `json:"currencyList"`
		} `json:"rates"`
	}
	decoder := json.NewDecoder(strings.NewReader(string(payload)))
	decoder.UseNumber()
	if err := decoder.Decode(&document); err != nil {
		return nil, fmt.Errorf("decode Raiffeisen rates: %w", err)
	}
	snapshots := make([]domain.RateSnapshot, 0, 4)
	for _, list := range document.Rates {
		for _, rate := range list.CurrencyList {
			currency := domain.NormalizeCurrency(rate.CurrencyPair.Left.Code)
			if !domain.ValidCurrency(currency) || currency == "RON" {
				continue
			}
			buy, buyErr := decimal.NewFromString(rate.BuyRate.Value.String())
			sell, sellErr := decimal.NewFromString(rate.SellRate.Value.String())
			if buyErr != nil || sellErr != nil || !validRetailQuote(buy, sell) {
				return nil, fmt.Errorf("invalid Raiffeisen %s quote", currency)
			}
			effectiveAt := fetchedAt
			if parsed, err := time.Parse(time.RFC3339Nano, rate.CreatedAt); err == nil {
				effectiveAt = parsed
			}
			snapshots = append(snapshots, retailSnapshot("raiffeisen", currency, buy, sell, RaiffeisenSourceURL, effectiveAt, fetchedAt))
		}
	}
	return requireFourRetailSnapshots("Raiffeisen", snapshots)
}

type INGProvider struct{ client *http.Client }

func NewINGProvider(client *http.Client) *INGProvider { return &INGProvider{client: client} }
func (p *INGProvider) ID() string                     { return "ing" }

func (p *INGProvider) Fetch(ctx context.Context) ([]domain.RateSnapshot, error) {
	payload, err := fetchPublicPayload(ctx, p.client, INGSourceURL, "ING rate page")
	if err != nil {
		return nil, err
	}
	return ParseINGRates(string(payload), time.Now().UTC())
}

var ingRatePattern = regexp.MustCompile(`(?s)\{"code"\s*:\s*"([A-Z]{3})"[^{}]*,"sell"\s*:\s*\{[^{}]*"promo1"\s*:\s*"([0-9.]+)"[^{}]*"promo2"\s*:\s*"([0-9.]+)"[^{}]*\},"buy"\s*:\s*\{[^{}]*"promo1"\s*:\s*"([0-9.]+)"[^{}]*"promo2"\s*:\s*"([0-9.]+)"[^{}]*\}`)

// ING publishes both its advantageous (promo1) and standard (promo2) account
// rates. They are persisted separately because the advantageous quote has
// package and monthly-volume conditions.
func ParseINGRates(document string, fetchedAt time.Time) ([]domain.RateSnapshot, error) {
	matches := ingRatePattern.FindAllStringSubmatch(document, -1)
	snapshots := make([]domain.RateSnapshot, 0, 8)
	for _, match := range matches {
		currency := domain.NormalizeCurrency(match[1])
		if !domain.ValidCurrency(currency) || currency == "RON" {
			continue
		}
		preferentialSell, preferentialSellErr := decimal.NewFromString(match[2])
		standardSell, standardSellErr := decimal.NewFromString(match[3])
		preferentialBuy, preferentialBuyErr := decimal.NewFromString(match[4])
		standardBuy, standardBuyErr := decimal.NewFromString(match[5])
		if preferentialSellErr != nil || standardSellErr != nil || preferentialBuyErr != nil || standardBuyErr != nil ||
			!validRetailQuote(preferentialBuy, preferentialSell) || !validRetailQuote(standardBuy, standardSell) {
			return nil, fmt.Errorf("invalid ING %s quote", currency)
		}
		snapshots = append(snapshots,
			retailSnapshot("ing", currency, standardBuy, standardSell, INGSourceURL, fetchedAt, fetchedAt),
			retailSnapshot("ing_preferential", currency, preferentialBuy, preferentialSell, INGSourceURL, fetchedAt, fetchedAt),
		)
	}
	return requireFourRetailSnapshots("ING", snapshots)
}

type CECProvider struct{ client *http.Client }

func NewCECProvider(client *http.Client) *CECProvider { return &CECProvider{client: client} }
func (p *CECProvider) ID() string                     { return "cec" }

func (p *CECProvider) Fetch(ctx context.Context) ([]domain.RateSnapshot, error) {
	payload, err := fetchPublicPayload(ctx, p.client, CECSourceURL, "CEC online rate page")
	if err != nil {
		return nil, err
	}
	return ParseCECRates(string(payload), time.Now().UTC())
}

// ParseCECRates reads CEC's public online-banking table where the numeric
// columns are BNR reference, bank buy, bank sell, ECB reference and margin.
func ParseCECRates(document string, fetchedAt time.Time) ([]domain.RateSnapshot, error) {
	plainText := normalizeHTML(document)
	snapshots := make([]domain.RateSnapshot, 0, 4)
	for _, currency := range []string{"EUR", "USD", "GBP", "CHF"} {
		position := currencyPosition(plainText, currency)
		if position < 0 {
			return nil, fmt.Errorf("CEC %s is missing", currency)
		}
		window := plainText[position:]
		if len(window) > 300 {
			window = window[:300]
		}
		values := numberPattern.FindAllString(window, 4)
		if len(values) < 3 {
			return nil, fmt.Errorf("CEC %s does not have reference/buy/sell values", currency)
		}
		buy, buyErr := parseRate(values[1])
		sell, sellErr := parseRate(values[2])
		if buyErr != nil || sellErr != nil || !validRetailQuote(buy, sell) {
			return nil, fmt.Errorf("invalid CEC %s quote", currency)
		}
		snapshots = append(snapshots, retailSnapshot("cec", currency, buy, sell, CECSourceURL, fetchedAt, fetchedAt))
	}
	return requireFourRetailSnapshots("CEC", snapshots)
}

func fetchPublicPayload(ctx context.Context, client *http.Client, url, source string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept-Language", "ro-RO,ro;q=0.9,en;q=0.8")
	request.Header.Set("User-Agent", "UndeSchimb/1.0 (+https://github.com/undeschimb/undeschimb)")
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", source, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s returned status %d", source, response.StatusCode)
	}
	payload, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	return payload, nil
}

func retailSnapshot(provider, currency string, buy, sell decimal.Decimal, sourceURL string, effectiveAt, fetchedAt time.Time) domain.RateSnapshot {
	return domain.RateSnapshot{
		Provider: provider, Currency: currency, BuyRate: buy, SellRate: sell,
		FeePercent: decimal.Zero, SourceURL: sourceURL, EffectiveAt: effectiveAt, FetchedAt: fetchedAt,
	}
}

func validRetailQuote(buy, sell decimal.Decimal) bool {
	return buy.IsPositive() && sell.IsPositive() && !buy.GreaterThan(sell)
}

func requireFourRetailSnapshots(provider string, snapshots []domain.RateSnapshot) ([]domain.RateSnapshot, error) {
	seen := make(map[string]struct{}, 4)
	for _, snapshot := range snapshots {
		seen[snapshot.Currency] = struct{}{}
	}
	if len(seen) != 4 {
		return nil, fmt.Errorf("%s feed did not contain all supported currencies", provider)
	}
	return snapshots, nil
}
