package providers

import (
	"context"
	"crypto/tls"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/shopspring/decimal"
	"github.com/undeschimb/undeschimb/internal/domain"
)

const BNRURL = "https://curs.bnr.ro/nbrfxrates.xml"

const (
	bnrFetchAttempts  = 3
	bnrAttemptTimeout = 6 * time.Second
	bnrRetryDelay     = 250 * time.Millisecond
)

type BNRProvider struct {
	client *http.Client
}

func NewBNRProvider(client *http.Client) *BNRProvider {
	bnrClient := *client
	transport := client.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	if transport, ok := transport.(*http.Transport); ok {
		bnrTransport := transport.Clone()
		if bnrTransport.TLSClientConfig == nil {
			bnrTransport.TLSClientConfig = &tls.Config{}
		}
		// Go 1.24's default hybrid key share makes ClientHello large enough to
		// trigger handshake timeouts on some TLS servers or network appliances.
		// Keep this compatibility setting local to BNR, with certificate
		// verification and TLS version negotiation unchanged.
		bnrTransport.TLSClientConfig.CurvePreferences = []tls.CurveID{tls.X25519, tls.CurveP256, tls.CurveP384}
		bnrClient.Transport = bnrTransport
	}
	return &BNRProvider{client: &bnrClient}
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
	for attempt := 1; ; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("fetch BNR XML: %w", err)
		}
		snapshots, retry, err := p.fetchAttempt(ctx)
		if err == nil {
			return snapshots, nil
		}
		if !retry || attempt == bnrFetchAttempts {
			return nil, fmt.Errorf("fetch BNR XML (attempt %d/%d): %w", attempt, bnrFetchAttempts, err)
		}
		timer := time.NewTimer(bnrRetryDelay * time.Duration(attempt))
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, fmt.Errorf("fetch BNR XML: %w", ctx.Err())
		case <-timer.C:
		}
	}
}

func (p *BNRProvider) fetchAttempt(ctx context.Context) ([]domain.RateSnapshot, bool, error) {
	// Leave room for all three attempts within the collector's 20-second limit.
	ctx, cancel := context.WithTimeout(ctx, bnrAttemptTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, BNRURL, nil)
	if err != nil {
		return nil, false, err
	}
	request.Header.Set("User-Agent", "UndeSchimb/1.0 (+https://github.com/undeschimb/undeschimb)")
	request.Header.Set("Accept", "application/xml, text/xml;q=0.9")
	response, err := p.client.Do(request)
	if err != nil {
		return nil, true, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		retry := response.StatusCode == http.StatusRequestTimeout || response.StatusCode >= http.StatusInternalServerError
		return nil, retry, fmt.Errorf("BNR XML returned status %d", response.StatusCode)
	}
	payload, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if err != nil {
		return nil, true, fmt.Errorf("read BNR XML: %w", err)
	}
	snapshots, err := ParseBNRXML(payload, time.Now().UTC())
	return snapshots, false, err
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
