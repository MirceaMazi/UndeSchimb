package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/shopspring/decimal"
	"github.com/undeschimb/undeschimb/internal/domain"
)

const (
	RevolutSourceURL = "https://www.revolut.com/ro-RO/currency-converter/"
	revolutAPIURL    = "https://www.revolut.com/api/exchange"
)

type RevolutProvider struct {
	client  *http.Client
	baseURL string
}

func NewRevolutProvider(client *http.Client) *RevolutProvider {
	return &RevolutProvider{client: client, baseURL: revolutAPIURL}
}

func (p *RevolutProvider) ID() string { return "revolut" }

type revolutMoney struct {
	Amount   int64  `json:"amount"`
	Currency string `json:"currency"`
}

type revolutQuoteResponse struct {
	Sender    revolutMoney `json:"sender"`
	Recipient revolutMoney `json:"recipient"`
	Rate      struct {
		From      string          `json:"from"`
		To        string          `json:"to"`
		Rate      decimal.Decimal `json:"rate"`
		Timestamp int64           `json:"timestamp"`
	} `json:"rate"`
	Plans []struct {
		ID   string `json:"id"`
		Fees struct {
			Total revolutMoney `json:"total"`
		} `json:"fees"`
	} `json:"plans"`
}

type revolutQuote struct {
	rate       decimal.Decimal
	feePercent decimal.Decimal
	effective  time.Time
}

func (p *RevolutProvider) Fetch(ctx context.Context) ([]domain.RateSnapshot, error) {
	fetchedAt := time.Now().UTC()
	snapshots := make([]domain.RateSnapshot, 0, 4)
	for _, currency := range []string{"EUR", "USD", "GBP", "CHF"} {
		// The public converter uses minor currency units. Reference amounts remain
		// inside the Standard plan's monthly allowance and avoid rounding noise.
		foreignToRON, err := p.fetchQuote(ctx, currency, "RON", 10_000)
		if err != nil {
			return nil, fmt.Errorf("read Revolut %s to RON quote: %w", currency, err)
		}
		ronToForeign, err := p.fetchQuote(ctx, "RON", currency, 100_000)
		if err != nil {
			return nil, fmt.Errorf("read Revolut RON to %s quote: %w", currency, err)
		}

		buyFee := foreignToRON.feePercent.Div(decimal.NewFromInt(100))
		sellFee := ronToForeign.feePercent.Div(decimal.NewFromInt(100))
		buyRate := foreignToRON.rate.Div(decimal.NewFromInt(1).Add(buyFee))
		sellRate := decimal.NewFromInt(1).Div(ronToForeign.rate).Mul(decimal.NewFromInt(1).Add(sellFee))
		feePercent := decimal.Max(foreignToRON.feePercent, ronToForeign.feePercent)
		effectiveAt := foreignToRON.effective
		if ronToForeign.effective.After(effectiveAt) {
			effectiveAt = ronToForeign.effective
		}
		if !validRetailQuote(buyRate, sellRate) {
			return nil, fmt.Errorf("Revolut returned an invalid %s buy/sell quote", currency)
		}
		snapshots = append(snapshots, domain.RateSnapshot{
			Provider: "revolut", Currency: currency, BuyRate: buyRate, SellRate: sellRate,
			FeePercent: feePercent, SourceURL: RevolutSourceURL,
			EffectiveAt: effectiveAt, FetchedAt: fetchedAt,
		})
	}
	return snapshots, nil
}

func (p *RevolutProvider) fetchQuote(ctx context.Context, from, to string, amount int64) (revolutQuote, error) {
	query := url.Values{
		"country":           {"RO"},
		"fromCurrency":      {from},
		"toCurrency":        {to},
		"amount":            {fmt.Sprintf("%d", amount)},
		"isRecipientAmount": {"false"},
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/quote?"+query.Encode(), nil)
	if err != nil {
		return revolutQuote{}, err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Accept-Language", "ro-RO,ro;q=0.9")
	request.Header.Set("User-Agent", "UndeSchimb/1.0 (+https://github.com/undeschimb/undeschimb)")
	response, err := p.client.Do(request)
	if err != nil {
		return revolutQuote{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return revolutQuote{}, fmt.Errorf("quote endpoint returned status %d", response.StatusCode)
	}
	var payload revolutQuoteResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return revolutQuote{}, fmt.Errorf("decode quote: %w", err)
	}
	if payload.Rate.From != from || payload.Rate.To != to || !payload.Rate.Rate.IsPositive() || payload.Rate.Timestamp <= 0 || payload.Sender.Amount <= 0 {
		return revolutQuote{}, fmt.Errorf("quote response is incomplete")
	}
	feePercent := decimal.Zero
	for _, plan := range payload.Plans {
		if plan.ID != "STANDARD" {
			continue
		}
		if plan.Fees.Total.Currency != "" && plan.Fees.Total.Currency != payload.Sender.Currency {
			return revolutQuote{}, fmt.Errorf("Standard fee currency does not match sender currency")
		}
		feePercent = decimal.NewFromInt(plan.Fees.Total.Amount).
			Div(decimal.NewFromInt(payload.Sender.Amount)).
			Mul(decimal.NewFromInt(100))
		break
	}
	return revolutQuote{
		rate: payload.Rate.Rate, feePercent: feePercent,
		effective: time.UnixMilli(payload.Rate.Timestamp).UTC(),
	}, nil
}
