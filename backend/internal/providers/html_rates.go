package providers

import (
	"context"
	"fmt"
	"html"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"github.com/undeschimb/undeschimb/internal/domain"
)

type BankPageProvider struct {
	id        string
	url       string
	markers   []string
	sectionID string
	client    *http.Client
}

func NewBankPageProvider(id, url string, markers []string, client *http.Client) *BankPageProvider {
	return &BankPageProvider{id: id, url: url, markers: markers, client: client}
}

func NewSectionBankPageProvider(id, url, sectionID string, client *http.Client) *BankPageProvider {
	return &BankPageProvider{id: id, url: url, sectionID: sectionID, client: client}
}

func (p *BankPageProvider) ID() string { return p.id }

func (p *BankPageProvider) Fetch(ctx context.Context) ([]domain.RateSnapshot, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, p.url, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept-Language", "ro-RO,ro;q=0.9,en;q=0.8")
	request.Header.Set("User-Agent", "UndeSchimb/1.0 (+https://github.com/undeschimb/undeschimb)")
	response, err := p.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("fetch %s exchange page: %w", p.id, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s exchange page returned status %d", p.id, response.StatusCode)
	}
	payload, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	var quotes map[string]accountQuote
	if p.sectionID != "" {
		quotes, err = ExtractSectionAccountRates(string(payload), p.sectionID)
	} else {
		quotes, err = ExtractAccountRates(string(payload), p.markers)
	}
	if err != nil {
		return nil, fmt.Errorf("parse %s account rates: %w", p.id, err)
	}
	now := time.Now().UTC()
	snapshots := make([]domain.RateSnapshot, 0, len(quotes))
	for currency, quote := range quotes {
		snapshots = append(snapshots, domain.RateSnapshot{
			Provider: p.id, Currency: currency, BuyRate: quote.Buy, SellRate: quote.Sell,
			FeePercent: decimal.Zero, SourceURL: p.url, EffectiveAt: now, FetchedAt: now,
		})
	}
	return snapshots, nil
}

type accountQuote struct {
	Buy  decimal.Decimal
	Sell decimal.Decimal
}

var tagPattern = regexp.MustCompile(`(?is)<[^>]+>`)
var whitespacePattern = regexp.MustCompile(`\s+`)
var numberPattern = regexp.MustCompile(`\b\d{1,2}[.,]\d{3,6}\b`)

// ExtractAccountRates reads the first two decimal values after each currency in an
// account/online section. Romanian bank pages label these columns Cumpărare then
// Vânzare, so the result is normalized into provider buy/sell terminology.
func ExtractAccountRates(document string, markers []string) (map[string]accountQuote, error) {
	if quotes, found := extractColumnarAccountRates(accountSection(document)); found {
		return quotes, nil
	}
	plainText := normalizeHTML(document)
	area := selectAccountArea(plainText, markers)
	quotes := make(map[string]accountQuote, 4)
	for _, currency := range []string{"EUR", "USD", "GBP", "CHF"} {
		position := currencyPosition(area, currency)
		if position < 0 {
			return nil, fmt.Errorf("%s is missing", currency)
		}
		window := area[position:]
		if len(window) > 260 {
			window = window[:260]
		}
		matches := numberPattern.FindAllString(window, 3)
		if len(matches) < 2 {
			return nil, fmt.Errorf("%s does not have buy/sell values", currency)
		}
		buy, err := parseRate(matches[0])
		if err != nil {
			return nil, err
		}
		sell, err := parseRate(matches[1])
		if err != nil {
			return nil, err
		}
		if !buy.IsPositive() || !sell.IsPositive() || buy.GreaterThan(sell) {
			return nil, fmt.Errorf("%s has invalid buy/sell ordering", currency)
		}
		quotes[currency] = accountQuote{Buy: buy, Sell: sell}
	}
	return quotes, nil
}

// ExtractSectionAccountRates parses a named public rate tab. BRD exposes its
// standard account and preferential YOU quotes as separate columnar sections.
func ExtractSectionAccountRates(document, sectionID string) (map[string]accountQuote, error) {
	section := sectionByID(document, sectionID)
	if section == "" {
		return nil, fmt.Errorf("section %s is missing", sectionID)
	}
	if quotes, found := extractColumnarAccountRates(section); found {
		return quotes, nil
	}
	return nil, fmt.Errorf("section %s does not contain four valid quotes", sectionID)
}

// Some banks publish their account table as parallel currency, buy, and sell
// columns instead of rows (notably BRD). Parse that structure before using the
// text-table fallback, so the NBR column is never mistaken for a bank quote.
func extractColumnarAccountRates(section string) (map[string]accountQuote, bool) {
	codesColumn, foundCodes := columnByHeading(section, "cod valut")
	buyColumn, foundBuy := columnByHeading(section, "cump")
	sellColumn, foundSell := columnByHeading(section, "vânz", "vanz")
	if !foundCodes || !foundBuy || !foundSell {
		return nil, false
	}
	codePattern := regexp.MustCompile(`\b(EUR|USD|GBP|CHF|AUD|CAD|CZK|DKK|HUF|JPY|NOK|PLN|SEK|MDL|TRY|CNY)\b`)
	codes := codePattern.FindAllString(strings.ToUpper(normalizeHTML(codesColumn)), -1)
	buys := numberPattern.FindAllString(normalizeHTML(buyColumn), -1)
	sells := numberPattern.FindAllString(normalizeHTML(sellColumn), -1)
	if len(codes) == 0 || len(codes) != len(buys) || len(codes) != len(sells) {
		return nil, false
	}
	quotes := make(map[string]accountQuote, 4)
	for index, code := range codes {
		if code != "EUR" && code != "USD" && code != "GBP" && code != "CHF" {
			continue
		}
		buy, buyErr := parseRate(buys[index])
		sell, sellErr := parseRate(sells[index])
		if buyErr != nil || sellErr != nil || !buy.IsPositive() || buy.GreaterThan(sell) {
			return nil, false
		}
		quotes[code] = accountQuote{Buy: buy, Sell: sell}
	}
	return quotes, len(quotes) == 4
}

func accountSection(document string) string {
	section := sectionByID(document, "tabAccountExchangeRates")
	if section != "" {
		return section
	}
	return document
}

func sectionByID(document, sectionID string) string {
	lower := strings.ToLower(document)
	start := strings.Index(lower, `id="`+strings.ToLower(sectionID)+`"`)
	if start < 0 {
		return ""
	}
	endOffset := strings.Index(lower[start+1:], `id="tab`)
	if endOffset < 0 {
		return document[start:]
	}
	return document[start : start+1+endOffset]
}

func columnByHeading(section string, headings ...string) (string, bool) {
	headingPattern := regexp.MustCompile(`(?is)<p[^>]*class\s*=\s*["'][^"']*heading[^"']*["'][^>]*>(.*?)</p>`)
	for _, match := range headingPattern.FindAllStringSubmatchIndex(section, -1) {
		heading := strings.ToLower(normalizeHTML(section[match[2]:match[3]]))
		matchesHeading := false
		for _, candidate := range headings {
			if strings.Contains(heading, strings.ToLower(candidate)) {
				matchesHeading = true
				break
			}
		}
		if !matchesHeading {
			continue
		}
		rest := section[match[1]:]
		if end := strings.Index(strings.ToLower(rest), "</div>"); end >= 0 {
			return rest[:end], true
		}
	}
	return "", false
}

func normalizeHTML(document string) string {
	document = html.UnescapeString(document)
	document = tagPattern.ReplaceAllString(document, " ")
	return whitespacePattern.ReplaceAllString(document, " ")
}

func selectAccountArea(document string, markers []string) string {
	lower := strings.ToLower(document)
	for _, marker := range markers {
		if index := strings.Index(lower, strings.ToLower(marker)); index >= 0 {
			end := index + 12000
			if end > len(document) {
				end = len(document)
			}
			return document[index:end]
		}
	}
	return document
}

func currencyPosition(document, currency string) int {
	pattern := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(currency) + `\b`)
	match := pattern.FindStringIndex(document)
	if match == nil {
		return -1
	}
	return match[0]
}

func parseRate(raw string) (decimal.Decimal, error) {
	return decimal.NewFromString(strings.ReplaceAll(raw, ",", "."))
}
