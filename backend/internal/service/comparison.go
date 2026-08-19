package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"github.com/undeschimb/undeschimb/internal/domain"
)

const staleAfter = 30 * time.Minute

type RateReader interface {
	LatestRates(context.Context, string) ([]domain.RateSnapshot, error)
	History(context.Context, string, string, time.Time) ([]domain.RateSnapshot, error)
}

type ComparisonService struct {
	rates RateReader
	now   func() time.Time
}

func NewComparisonService(rates RateReader) *ComparisonService {
	return &ComparisonService{rates: rates, now: time.Now}
}

type ComparisonRequest struct {
	From   string
	To     string
	Amount decimal.Decimal
}

type ComparisonResponse struct {
	From           string          `json:"from"`
	To             string          `json:"to"`
	InputAmount    decimal.Decimal `json:"input_amount"`
	BNROutput      decimal.Decimal `json:"bnr_output"`
	BNRRate        decimal.Decimal `json:"bnr_rate"`
	BNRFetchedAt   time.Time       `json:"bnr_fetched_at"`
	Offers         []Offer         `json:"offers"`
	MissingSources []string        `json:"missing_sources"`
}

type Offer struct {
	Provider               string          `json:"provider"`
	ProviderName           string          `json:"provider_name"`
	EffectiveRate          decimal.Decimal `json:"effective_rate"`
	OutputAmount           decimal.Decimal `json:"output_amount"`
	DifferenceFromBNR      decimal.Decimal `json:"difference_from_bnr"`
	DifferenceFromBNRInRON decimal.Decimal `json:"difference_from_bnr_ron"`
	DifferencePercent      decimal.Decimal `json:"difference_percent"`
	FeePercent             decimal.Decimal `json:"fee_percent"`
	SourceURL              string          `json:"source_url"`
	FetchedAt              time.Time       `json:"fetched_at"`
	EffectiveAt            time.Time       `json:"effective_at"`
	Stale                  bool            `json:"stale"`
	Indicative             bool            `json:"indicative"`
}

func (s *ComparisonService) Compare(ctx context.Context, request ComparisonRequest) (ComparisonResponse, error) {
	request.From = domain.NormalizeCurrency(request.From)
	request.To = domain.NormalizeCurrency(request.To)
	if request.Amount.LessThanOrEqual(decimal.Zero) {
		return ComparisonResponse{}, fmt.Errorf("amount must be greater than zero")
	}
	if !((request.From == "RON" && request.To != "RON") || (request.To == "RON" && request.From != "RON")) {
		return ComparisonResponse{}, fmt.Errorf("only RON to/from EUR, USD, GBP, or CHF is supported")
	}
	currency := request.To
	if currency == "RON" {
		currency = request.From
	}
	snapshots, err := s.rates.LatestRates(ctx, currency)
	if err != nil {
		return ComparisonResponse{}, err
	}
	var bnr *domain.RateSnapshot
	byProvider := make(map[string]domain.RateSnapshot)
	for _, snapshot := range snapshots {
		if snapshot.Provider == "bnr" {
			copy := snapshot
			bnr = &copy
			continue
		}
		byProvider[snapshot.Provider] = snapshot
	}
	if bnr == nil {
		return ComparisonResponse{}, fmt.Errorf("BNR reference rate is not available for %s", currency)
	}
	bnrOutput := calculateOutput(request.From, request.Amount, bnr)
	response := ComparisonResponse{
		From: request.From, To: request.To, InputAmount: request.Amount, BNROutput: bnrOutput,
		BNRRate: rateForDirection(request.From, bnr), BNRFetchedAt: bnr.FetchedAt,
		Offers: make([]Offer, 0, 7), MissingSources: make([]string, 0, 7),
	}
	for _, provider := range []string{"banca_transilvania", "bcr", "brd", "ing", "raiffeisen", "cec", "xtb"} {
		snapshot, found := byProvider[provider]
		if !found {
			response.MissingSources = append(response.MissingSources, provider)
			continue
		}
		output := calculateOutput(request.From, request.Amount, &snapshot)
		difference := output.Sub(bnrOutput)
		differenceInRON := difference
		if request.From == "RON" {
			differenceInRON = difference.Mul(bnr.SellRate)
		}
		percentage := decimal.Zero
		if !bnrOutput.IsZero() {
			percentage = difference.Div(bnrOutput).Mul(decimal.NewFromInt(100))
		}
		response.Offers = append(response.Offers, Offer{
			Provider: provider, ProviderName: domain.ProviderNames[provider], EffectiveRate: rateForDirection(request.From, &snapshot),
			OutputAmount: output, DifferenceFromBNR: difference, DifferenceFromBNRInRON: differenceInRON,
			DifferencePercent: percentage, FeePercent: snapshot.FeePercent, SourceURL: snapshot.SourceURL,
			FetchedAt: snapshot.FetchedAt, EffectiveAt: snapshot.EffectiveAt,
			Stale: s.now().UTC().Sub(snapshot.FetchedAt) > staleAfter, Indicative: provider == "xtb",
		})
	}
	sort.Slice(response.Offers, func(i, j int) bool {
		return response.Offers[i].OutputAmount.GreaterThan(response.Offers[j].OutputAmount)
	})
	return response, nil
}

func calculateOutput(from string, amount decimal.Decimal, snapshot *domain.RateSnapshot) decimal.Decimal {
	if from == "RON" {
		return amount.Div(snapshot.SellRate)
	}
	return amount.Mul(snapshot.BuyRate)
}

func rateForDirection(from string, snapshot *domain.RateSnapshot) decimal.Decimal {
	if from == "RON" {
		return snapshot.SellRate
	}
	return snapshot.BuyRate
}

type HistoryPoint struct {
	Date         string          `json:"date"`
	ProviderRate decimal.Decimal `json:"provider_rate"`
	BNRRate      decimal.Decimal `json:"bnr_rate"`
}

type HistoryResponse struct {
	Provider string         `json:"provider"`
	Currency string         `json:"currency"`
	Side     string         `json:"side"`
	Points   []HistoryPoint `json:"points"`
}

func (s *ComparisonService) History(ctx context.Context, provider, currency, side string, days int) (HistoryResponse, error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	currency = domain.NormalizeCurrency(currency)
	side = strings.ToLower(strings.TrimSpace(side))
	if _, exists := domain.ProviderNames[provider]; !exists || provider == "bnr" {
		return HistoryResponse{}, fmt.Errorf("unknown provider")
	}
	if currency == "RON" || !domain.ValidCurrency(currency) {
		return HistoryResponse{}, fmt.Errorf("unsupported currency")
	}
	if side != "buy" && side != "sell" {
		return HistoryResponse{}, fmt.Errorf("side must be buy or sell")
	}
	if days != 7 && days != 30 && days != 90 {
		return HistoryResponse{}, fmt.Errorf("period must be 7, 30, or 90 days")
	}
	since := s.now().UTC().AddDate(0, 0, -days)
	providerSnapshots, err := s.rates.History(ctx, provider, currency, since)
	if err != nil {
		return HistoryResponse{}, err
	}
	bnrSnapshots, err := s.rates.History(ctx, "bnr", currency, since)
	if err != nil {
		return HistoryResponse{}, err
	}
	providerByDate := latestByDate(providerSnapshots, side)
	bnrByDate := latestByDate(bnrSnapshots, side)
	points := make([]HistoryPoint, 0, len(providerByDate))
	for date, providerRate := range providerByDate {
		if bnrRate, exists := bnrByDate[date]; exists {
			points = append(points, HistoryPoint{Date: date, ProviderRate: providerRate, BNRRate: bnrRate})
		}
	}
	sort.Slice(points, func(i, j int) bool { return points[i].Date < points[j].Date })
	return HistoryResponse{Provider: provider, Currency: currency, Side: side, Points: points}, nil
}

func latestByDate(snapshots []domain.RateSnapshot, side string) map[string]decimal.Decimal {
	result := make(map[string]decimal.Decimal)
	for _, snapshot := range snapshots {
		date := snapshot.FetchedAt.In(bucharest()).Format("2006-01-02")
		result[date] = snapshot.BuyRate
		if side == "sell" {
			result[date] = snapshot.SellRate
		}
	}
	return result
}

func bucharest() *time.Location {
	location, err := time.LoadLocation("Europe/Bucharest")
	if err != nil {
		return time.UTC
	}
	return location
}
