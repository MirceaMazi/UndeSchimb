package providers

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"github.com/undeschimb/undeschimb/internal/domain"
)

const TavexSourceURL = "https://tavex.ro/schimb-valutar/"

type TavexProvider struct{ client *http.Client }

func NewTavexProvider(client *http.Client) *TavexProvider { return &TavexProvider{client: client} }
func (p *TavexProvider) ID() string                       { return "tavex" }

func (p *TavexProvider) Fetch(ctx context.Context) ([]domain.RateSnapshot, error) {
	payload, err := fetchPublicPayload(ctx, p.client, TavexSourceURL, "Tavex exchange-rate page")
	if err != nil {
		return nil, err
	}
	return ParseTavexRates(string(payload), time.Now().UTC())
}

var tavexRowPattern = regexp.MustCompile(`(?is)<tr[^>]*class\s*=\s*["'][^"']*list-table__row[^"']*["'][^>]*>.*?</tr>`)
var tavexCodePattern = regexp.MustCompile(`(?is)<abbr[^>]*>\s*(EUR|USD|GBP|CHF)\s*</abbr>`)
var tavexValuePattern = regexp.MustCompile(`(?is)<td[^>]*class\s*=\s*["'][^"']*list-table__col--value[^"']*["'][^>]*>\s*([0-9]+(?:[.,][0-9]{2,6})?)\s*</td>`)

func ParseTavexRates(document string, fetchedAt time.Time) ([]domain.RateSnapshot, error) {
	snapshots := make([]domain.RateSnapshot, 0, 4)
	seen := make(map[string]struct{}, 4)
	for _, row := range tavexRowPattern.FindAllString(document, -1) {
		codeMatch := tavexCodePattern.FindStringSubmatch(row)
		if len(codeMatch) != 2 {
			continue
		}
		currency := strings.ToUpper(codeMatch[1])
		if _, exists := seen[currency]; exists {
			continue
		}
		values := tavexValuePattern.FindAllStringSubmatch(row, 2)
		if len(values) != 2 {
			return nil, fmt.Errorf("Tavex %s row does not contain buy and sell rates", currency)
		}
		buy, buyErr := parseRate(values[0][1])
		sell, sellErr := parseRate(values[1][1])
		if buyErr != nil || sellErr != nil || !validRetailQuote(buy, sell) {
			return nil, fmt.Errorf("invalid Tavex %s quote", currency)
		}
		snapshots = append(snapshots, domain.RateSnapshot{
			Provider: "tavex", Currency: currency, BuyRate: buy, SellRate: sell,
			FeePercent: decimal.Zero, SourceURL: TavexSourceURL, EffectiveAt: fetchedAt, FetchedAt: fetchedAt,
		})
		seen[currency] = struct{}{}
	}
	return requireFourRetailSnapshots("Tavex", snapshots)
}
