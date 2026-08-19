package providers

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/undeschimb/undeschimb/internal/domain"
)

const LuxorBucharestSourceURL = "https://www.luxor-exchange.ro/bucuresti"

type LuxorBucharestProvider struct{ client *http.Client }

func NewLuxorBucharestProvider(client *http.Client) *LuxorBucharestProvider {
	return &LuxorBucharestProvider{client: client}
}

func (p *LuxorBucharestProvider) ID() string { return "luxor_bucharest" }

func (p *LuxorBucharestProvider) Fetch(ctx context.Context) ([]domain.RateSnapshot, error) {
	payload, err := fetchPublicPayload(ctx, p.client, LuxorBucharestSourceURL, "Luxor Bucharest exchange-rate page")
	if err != nil {
		return nil, err
	}
	return ParseLuxorBucharestRates(string(payload), time.Now().UTC())
}

var luxorRowPattern = regexp.MustCompile(`(?is)<tr[^>]*>.*?<strong>\s*(EUR|USD|GBP|CHF)\s*</strong>.*?</tr>`)
var luxorValuePattern = regexp.MustCompile(`(?is)<td[^>]*>\s*([0-9]+(?:[.,][0-9]{2,6})?)\s*Lei\s*</td>`)

func ParseLuxorBucharestRates(document string, fetchedAt time.Time) ([]domain.RateSnapshot, error) {
	snapshots := make([]domain.RateSnapshot, 0, 4)
	seen := make(map[string]struct{}, 4)
	for _, row := range luxorRowPattern.FindAllString(document, -1) {
		codeMatch := luxorRowPattern.FindStringSubmatch(row)
		if len(codeMatch) != 2 {
			continue
		}
		currency := strings.ToUpper(codeMatch[1])
		if _, exists := seen[currency]; exists {
			continue
		}
		values := luxorValuePattern.FindAllStringSubmatch(row, 2)
		if len(values) != 2 {
			return nil, fmt.Errorf("Luxor București %s row does not contain buy and sell rates", currency)
		}
		buy, buyErr := parseRate(values[0][1])
		sell, sellErr := parseRate(values[1][1])
		if buyErr != nil || sellErr != nil || !validRetailQuote(buy, sell) {
			return nil, fmt.Errorf("invalid Luxor București %s quote", currency)
		}
		snapshots = append(snapshots, retailSnapshot(
			"luxor_bucharest", currency, buy, sell, LuxorBucharestSourceURL, fetchedAt, fetchedAt,
		))
		seen[currency] = struct{}{}
	}
	return requireFourRetailSnapshots("Luxor București", snapshots)
}
