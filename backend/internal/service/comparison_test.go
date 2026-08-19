package service

import (
	"context"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/undeschimb/undeschimb/internal/domain"
)

type fakeRateStore struct {
	latest  []domain.RateSnapshot
	history map[string][]domain.RateSnapshot
}

func (s fakeRateStore) LatestRates(_ context.Context, _ string) ([]domain.RateSnapshot, error) { return s.latest, nil }
func (s fakeRateStore) History(_ context.Context, provider, _ string, _ time.Time) ([]domain.RateSnapshot, error) {
	return s.history[provider], nil
}

func snapshot(provider string, buy, sell string) domain.RateSnapshot {
	return domain.RateSnapshot{
		Provider: provider, Currency: "EUR", BuyRate: decimal.RequireFromString(buy), SellRate: decimal.RequireFromString(sell),
		SourceURL: "https://example.test", FetchedAt: time.Date(2026, 8, 19, 10, 0, 0, 0, time.UTC),
	}
}

func TestCompareRONToCurrencyUsesSellRateAndRanksOutput(t *testing.T) {
	service := NewComparisonService(fakeRateStore{latest: []domain.RateSnapshot{
		snapshot("bnr", "5", "5"), snapshot("bcr", "4.9", "5.2"), snapshot("brd", "4.9", "5.1"),
	}})
	service.now = func() time.Time { return time.Date(2026, 8, 19, 10, 1, 0, 0, time.UTC) }
	response, err := service.Compare(context.Background(), ComparisonRequest{From: "RON", To: "EUR", Amount: decimal.NewFromInt(510)})
	if err != nil {
		t.Fatal(err)
	}
	if got := response.Offers[0].Provider; got != "brd" {
		t.Fatalf("expected BRD first, got %s", got)
	}
	if response.Offers[0].OutputAmount.StringFixed(2) != "100.00" {
		t.Fatalf("unexpected output: %s", response.Offers[0].OutputAmount)
	}
	if response.Offers[0].DifferenceFromBNRInRON.StringFixed(2) != "-10.00" {
		t.Fatalf("unexpected BNR difference: %s", response.Offers[0].DifferenceFromBNRInRON)
	}
}

func TestCompareCurrencyToRONUsesBuyRate(t *testing.T) {
	service := NewComparisonService(fakeRateStore{latest: []domain.RateSnapshot{
		snapshot("bnr", "5", "5"), snapshot("bcr", "4.8", "5.2"),
	}})
	response, err := service.Compare(context.Background(), ComparisonRequest{From: "EUR", To: "RON", Amount: decimal.NewFromInt(100)})
	if err != nil {
		t.Fatal(err)
	}
	if got := response.Offers[0].OutputAmount.StringFixed(2); got != "480.00" {
		t.Fatalf("expected 480 RON, got %s", got)
	}
	if got := response.Offers[0].DifferenceFromBNR.StringFixed(2); got != "-20.00" {
		t.Fatalf("expected -20 RON, got %s", got)
	}
}

func TestCompareMarksAnOldSnapshotStale(t *testing.T) {
	old := snapshot("bcr", "4.8", "5.2")
	old.FetchedAt = time.Date(2026, 8, 19, 9, 0, 0, 0, time.UTC)
	service := NewComparisonService(fakeRateStore{latest: []domain.RateSnapshot{snapshot("bnr", "5", "5"), old}})
	service.now = func() time.Time { return time.Date(2026, 8, 19, 10, 0, 0, 0, time.UTC) }
	response, err := service.Compare(context.Background(), ComparisonRequest{From: "RON", To: "EUR", Amount: decimal.NewFromInt(100)})
	if err != nil {
		t.Fatal(err)
	}
	if !response.Offers[0].Stale {
		t.Fatal("expected stale offer")
	}
}
