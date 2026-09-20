package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/undeschimb/undeschimb/internal/domain"
)

func TestHistoryKeepsRecentProviderRatesWithoutBNR(t *testing.T) {
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	var bankRates []domain.RateSnapshot
	for _, age := range []int{91, 31, 8, 6, 3, 1} {
		rate := snapshot("bcr", "5.1", "5.3")
		rate.FetchedAt = now.AddDate(0, 0, -age)
		bankRates = append(bankRates, rate)
	}
	oldBNR := snapshot("bnr", "5.2", "5.2")
	oldBNR.FetchedAt = now.AddDate(0, 0, -8)
	service := NewComparisonService(fakeRateStore{history: map[string][]domain.RateSnapshot{
		"bcr": bankRates, "bnr": {oldBNR},
	}})
	service.now = func() time.Time { return now }
	for _, period := range []struct{ days, count int }{{7, 3}, {30, 4}, {90, 5}} {
		for _, side := range []string{"buy", "sell"} {
			t.Run(fmt.Sprintf("%dd/%s", period.days, side), func(t *testing.T) {
				result, err := service.History(context.Background(), "bcr", "EUR", side, period.days)
				if err != nil {
					t.Fatal(err)
				}
				if len(result.Points) != period.count {
					t.Fatalf("got %d observations, want %d", len(result.Points), period.count)
				}
				for i, point := range result.Points {
					if i > 0 && result.Points[i-1].Date >= point.Date {
						t.Fatal("history must be sorted by date")
					}
					want := "5.3"
					if side == "buy" {
						want = "5.1"
					}
					if point.ProviderRate.String() != want {
						t.Fatalf("wrong %s rate: %s", side, point.ProviderRate)
					}
					if point.Date == "2026-09-13" {
						if point.BNRRate == nil || point.BNRRate.String() != "5.2" {
							t.Fatal("matching BNR rate should be retained")
						}
					} else if point.BNRRate != nil {
						t.Fatal("missing BNR must not be replaced by an older rate or zero")
					}
				}
				payload, err := json.Marshal(result)
				if err != nil || !strings.Contains(string(payload), `"bnr_rate":null`) {
					t.Fatalf("missing BNR should be JSON null: %s (%v)", payload, err)
				}
			})
		}
	}
}

func TestHistoryUsesLatestDailyRatesInBucharest(t *testing.T) {
	early := snapshot("bcr", "5.1", "5.3")
	early.FetchedAt = time.Date(2026, 9, 19, 21, 30, 0, 0, time.UTC)
	late := snapshot("bcr", "5.15", "5.35")
	late.FetchedAt = early.FetchedAt.Add(time.Hour)
	bnr := snapshot("bnr", "5.2", "5.2")
	bnr.FetchedAt = late.FetchedAt
	service := NewComparisonService(fakeRateStore{history: map[string][]domain.RateSnapshot{
		"bcr": {early, late}, "bnr": {bnr},
	}})
	service.now = func() time.Time { return late.FetchedAt.Add(time.Hour) }
	result, err := service.History(context.Background(), "bcr", "EUR", "sell", 7)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Points) != 1 || result.Points[0].Date != "2026-09-20" || result.Points[0].ProviderRate.String() != "5.35" {
		t.Fatalf("expected latest quote for the Romanian calendar day: %+v", result.Points)
	}
}

func TestHistoryDoesNotInventProviderObservations(t *testing.T) {
	bnr := snapshot("bnr", "5.2", "5.2")
	service := NewComparisonService(fakeRateStore{history: map[string][]domain.RateSnapshot{"bnr": {bnr}}})
	service.now = func() time.Time { return bnr.FetchedAt }
	result, err := service.History(context.Background(), "bcr", "EUR", "sell", 7)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(result)
	if err != nil || !strings.Contains(string(payload), `"points":[]`) {
		t.Fatalf("expected an empty array, not invented points: %s (%v)", payload, err)
	}
}
