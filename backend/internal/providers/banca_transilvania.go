package providers

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/shopspring/decimal"
	"github.com/undeschimb/undeschimb/internal/domain"
)

// BTXMLURL is Banca Transilvania's documented developer XML feed. It avoids
// scraping the consumer page and is cached by UndeSchimb between refreshes.
const BTXMLURL = "https://dev.bancatransilvania.ro/exchange.xml"

type BancaTransilvaniaProvider struct {
	client *http.Client
}

func NewBancaTransilvaniaProvider(client *http.Client) *BancaTransilvaniaProvider {
	return &BancaTransilvaniaProvider{client: client}
}

func (p *BancaTransilvaniaProvider) ID() string { return "banca_transilvania" }

type btDocument struct {
	UpdateDate struct {
		Value string `xml:"name,attr"`
	} `xml:"updateDate"`
	Rates []btCurrency `xml:"exchangeRates>currency"`
}

type btCurrency struct {
	Currency string `xml:"name,attr"`
	Buy struct {
		Value string `xml:"value"`
	} `xml:"buy"`
	Sell struct {
		Value string `xml:"value"`
	} `xml:"sell"`
}

func (p *BancaTransilvaniaProvider) Fetch(ctx context.Context) ([]domain.RateSnapshot, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, BTXMLURL, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("User-Agent", "UndeSchimb/1.0 (+https://github.com/undeschimb/undeschimb)")
	response, err := p.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("fetch Banca Transilvania XML: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Banca Transilvania XML returned status %d", response.StatusCode)
	}
	payload, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	return ParseBancaTransilvaniaXML(payload, time.Now().UTC())
}

func ParseBancaTransilvaniaXML(payload []byte, fetchedAt time.Time) ([]domain.RateSnapshot, error) {
	var document btDocument
	if err := xml.Unmarshal(payload, &document); err != nil {
		return nil, fmt.Errorf("parse Banca Transilvania XML: %w", err)
	}
	location := bucharestLocation()
	effectiveAt, err := time.ParseInLocation("2006-01-02 15:04:05", document.UpdateDate.Value, location)
	if err != nil {
		return nil, fmt.Errorf("parse Banca Transilvania update date: %w", err)
	}
	snapshots := make([]domain.RateSnapshot, 0, 4)
	for _, rate := range document.Rates {
		currency := domain.NormalizeCurrency(rate.Currency)
		if currency != "EUR" && currency != "USD" && currency != "GBP" && currency != "CHF" {
			continue
		}
		buy, buyErr := decimal.NewFromString(rate.Buy.Value)
		sell, sellErr := decimal.NewFromString(rate.Sell.Value)
		if buyErr != nil || sellErr != nil || !buy.IsPositive() || !sell.IsPositive() || buy.GreaterThan(sell) {
			return nil, fmt.Errorf("invalid Banca Transilvania %s quote", currency)
		}
		snapshots = append(snapshots, domain.RateSnapshot{
			Provider: "banca_transilvania", Currency: currency, BuyRate: buy, SellRate: sell,
			FeePercent: decimal.Zero, SourceURL: BTXMLURL, EffectiveAt: effectiveAt.UTC(), FetchedAt: fetchedAt,
		})
	}
	if len(snapshots) != 4 {
		return nil, fmt.Errorf("Banca Transilvania feed did not contain all supported currencies")
	}
	return snapshots, nil
}

