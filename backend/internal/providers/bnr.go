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

const BNRURL = "https://curs.bnr.ro/nbrfxrates.xml"

type BNRProvider struct {
	client *http.Client
}

func NewBNRProvider(client *http.Client) *BNRProvider {
	return &BNRProvider{client: client}
}

func (p *BNRProvider) ID() string { return "bnr" }

type bnrDocument struct {
	Body struct {
		Cube struct {
			Date  string    `xml:"date,attr"`
			Rates []bnrRate `xml:"Rate"`
		} `xml:"Cube"`
	} `xml:"Body"`
}

type bnrRate struct {
	Currency   string `xml:"currency,attr"`
	Multiplier string `xml:"multiplier,attr"`
	Value      string `xml:",chardata"`
}

func (p *BNRProvider) Fetch(ctx context.Context) ([]domain.RateSnapshot, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, BNRURL, nil)
	if err != nil {
		return nil, err
	}
	response, err := p.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("fetch BNR XML: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("BNR XML returned status %d", response.StatusCode)
	}
	payload, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	return ParseBNRXML(payload, time.Now().UTC())
}

func ParseBNRXML(payload []byte, fetchedAt time.Time) ([]domain.RateSnapshot, error) {
	var document bnrDocument
	if err := xml.Unmarshal(payload, &document); err != nil {
		return nil, fmt.Errorf("parse BNR XML: %w", err)
	}
	location, err := time.LoadLocation("Europe/Bucharest")
	if err != nil {
		location = time.UTC
	}
	effectiveAt, err := time.ParseInLocation("2006-01-02", document.Body.Cube.Date, location)
	if err != nil {
		return nil, fmt.Errorf("parse BNR rate date: %w", err)
	}
	snapshots := make([]domain.RateSnapshot, 0, 4)
	for _, rate := range document.Body.Cube.Rates {
		currency := domain.NormalizeCurrency(rate.Currency)
		if currency == "RON" || !domain.ValidCurrency(currency) {
			continue
		}
		value, err := decimal.NewFromString(rate.Value)
		if err != nil {
			return nil, fmt.Errorf("parse BNR %s rate: %w", currency, err)
		}
		multiplier := decimal.NewFromInt(1)
		if rate.Multiplier != "" {
			multiplier, err = decimal.NewFromString(rate.Multiplier)
			if err != nil {
				return nil, fmt.Errorf("parse BNR %s multiplier: %w", currency, err)
			}
		}
		perUnit := value.Div(multiplier)
		snapshots = append(snapshots, domain.RateSnapshot{
			Provider: "bnr", Currency: currency, BuyRate: perUnit, SellRate: perUnit,
			FeePercent: decimal.Zero, SourceURL: BNRURL, EffectiveAt: effectiveAt, FetchedAt: fetchedAt,
		})
	}
	if len(snapshots) != 4 {
		return nil, fmt.Errorf("BNR feed did not contain all supported currencies")
	}
	return snapshots, nil
}
