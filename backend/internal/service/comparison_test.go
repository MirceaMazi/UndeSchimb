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

func (s fakeRateStore) LatestRates(_ context.Context, _ string) ([]domain.RateSnapshot, error) {
	return s.latest, nil
}
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
	offer, found := findOffer(response.Offers, "brd")
	if !found {
		t.Fatal("expected a BRD offer")
	}
	if offer.OutputAmount.StringFixed(2) != "100.00" {
		t.Fatalf("unexpected output: %s", offer.OutputAmount)
	}
	if offer.DifferenceFromBNRInRON.StringFixed(2) != "-10.00" {
		t.Fatalf("unexpected BNR difference: %s", offer.DifferenceFromBNRInRON)
	}
}

func TestCompareCurrencyToRONUsesBuyRate(t *testing.T) {
	service := NewComparisonService(fakeRateStore{latest: []domain.RateSnapshot{
		snapshot("bnr", "5", "5"), snapshot("bcr", "4.8", "5.2"),
	}})
	service.now = func() time.Time { return time.Date(2026, 8, 19, 14, 0, 0, 0, time.UTC) }
	response, err := service.Compare(context.Background(), ComparisonRequest{From: "EUR", To: "RON", Amount: decimal.NewFromInt(100)})
	if err != nil {
		t.Fatal(err)
	}
	offer, found := findOffer(response.Offers, "bcr")
	if !found {
		t.Fatal("expected a BCR offer")
	}
	if got := offer.OutputAmount.StringFixed(2); got != "480.00" {
		t.Fatalf("expected 480 RON, got %s", got)
	}
	if got := offer.DifferenceFromBNR.StringFixed(2); got != "-20.00" {
		t.Fatalf("expected -20 RON, got %s", got)
	}
}

func TestCompareCategorizesProvidersAndKeepsINGPreferentialConditional(t *testing.T) {
	service := NewComparisonService(fakeRateStore{latest: []domain.RateSnapshot{
		snapshot("bnr", "5", "5"), snapshot("brd_you", "4.97", "5.03"), snapshot("ing", "4.8", "5.2"), snapshot("ing_preferential", "4.95", "5.05"), snapshot("xtb", "4.9", "5.1"), snapshot("revolut", "4.92", "5.08"), snapshot("tavex", "4.7", "5.3"), snapshot("luxor_bucharest", "4.8", "5.2"),
	}})
	service.now = func() time.Time { return time.Date(2026, 8, 19, 14, 0, 0, 0, time.UTC) }
	response, err := service.Compare(context.Background(), ComparisonRequest{From: "RON", To: "EUR", Amount: decimal.NewFromInt(3000)})
	if err != nil {
		t.Fatal(err)
	}
	byProvider := make(map[string]Offer)
	for _, offer := range response.Offers {
		byProvider[offer.Provider] = offer
	}
	if byProvider["xtb"].Category != domain.CategoryBrokers || byProvider["tavex"].Category != domain.CategoryPhysicalExchanges {
		t.Fatalf("unexpected provider categories: %#v", byProvider)
	}
	if offer := byProvider["revolut"]; offer.Category != domain.CategoryBrokers || offer.OfferType != "indicative" || !offer.Indicative {
		t.Fatalf("expected an indicative numeric Revolut offer, got %#v", offer)
	}
	if offer := byProvider["tavex"]; offer.LocationPolicy != "same_all_locations" || offer.LocationLabel == "" {
		t.Fatalf("expected Tavex all-location metadata, got %#v", offer)
	}
	if offer := byProvider["luxor_bucharest"]; offer.LocationPolicy != "varies_by_city" || offer.LocationNote == "" {
		t.Fatalf("expected Luxor city-specific metadata, got %#v", offer)
	}
	if offer := byProvider["ing_preferential"]; offer.OfferType != "preferential" || !offer.Conditional {
		t.Fatalf("expected conditional ING preferential offer, got %#v", offer)
	}
	if offer := byProvider["brd_you"]; offer.OfferType != "preferential" || !offer.Conditional {
		t.Fatalf("expected conditional BRD YOU offer, got %#v", offer)
	}
	if len(response.ProviderNotices) < 3 {
		t.Fatalf("expected ING, TradeVille, and Revolut notices, got %#v", response.ProviderNotices)
	}
	foundNotices := map[string]bool{}
	for _, notice := range response.ProviderNotices {
		foundNotices[notice.Provider] = true
	}
	for _, provider := range []string{"revolut", "brd_you", "banca_transilvania_negotiated", "bcr_preferential", "cec_digital"} {
		if !foundNotices[provider] {
			t.Fatalf("expected an audited %s benefit notice, got %#v", provider, response.ProviderNotices)
		}
	}
}

func TestLuxorIsRankedOnlyAbovePublished500EUREquivalentThreshold(t *testing.T) {
	service := NewComparisonService(fakeRateStore{latest: []domain.RateSnapshot{
		snapshot("bnr", "5", "5"), snapshot("luxor_bucharest", "4.9", "5.1"), snapshot("tavex", "4.7", "5.3"),
	}})
	service.now = func() time.Time { return time.Date(2026, 8, 19, 14, 0, 0, 0, time.UTC) }

	below, err := service.Compare(context.Background(), ComparisonRequest{From: "RON", To: "EUR", Amount: decimal.NewFromInt(2500)})
	if err != nil {
		t.Fatal(err)
	}
	for _, offer := range below.Offers {
		if offer.Provider == "luxor_bucharest" {
			t.Fatal("Luxor must not be ranked at or below 500 EUR equivalent")
		}
	}
	if !containsProvider(below.Offers, "tavex") {
		t.Fatal("Tavex standard rate must remain eligible without a minimum amount")
	}
	foundIneligibleNotice := false
	for _, notice := range below.ProviderNotices {
		if notice.Provider == "luxor_bucharest" && notice.Kind == "ineligible" && !notice.RequestEligible {
			foundIneligibleNotice = true
		}
	}
	if !foundIneligibleNotice {
		t.Fatalf("expected an ineligible Luxor notice, got %#v", below.ProviderNotices)
	}

	above, err := service.Compare(context.Background(), ComparisonRequest{From: "RON", To: "EUR", Amount: decimal.RequireFromString("2500.01")})
	if err != nil {
		t.Fatal(err)
	}
	if !containsProvider(above.Offers, "luxor_bucharest") {
		t.Fatal("Luxor should be ranked above 500 EUR equivalent")
	}
}

func containsProvider(offers []Offer, provider string) bool {
	for _, offer := range offers {
		if offer.Provider == provider {
			return true
		}
	}
	return false
}

func findOffer(offers []Offer, provider string) (Offer, bool) {
	for _, offer := range offers {
		if offer.Provider == provider {
			return offer, true
		}
	}
	return Offer{}, false
}

func TestRaiffeisenSmartHourExposesAnAvailabilityAwarePreview(t *testing.T) {
	store := fakeRateStore{latest: []domain.RateSnapshot{snapshot("bnr", "5", "5"), snapshot("raiffeisen", "4.8", "5.2")}}
	service := NewComparisonService(store)
	service.now = func() time.Time {
		// 10:30 in Bucharest during summer time, on a Wednesday.
		return time.Date(2026, 8, 19, 7, 30, 0, 0, time.UTC)
	}
	response, err := service.Compare(context.Background(), ComparisonRequest{From: "RON", To: "EUR", Amount: decimal.NewFromInt(500)})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, offer := range response.Offers {
		if offer.Provider == "raiffeisen_smart_hour" {
			found = true
			if !offer.ActiveNow || !offer.RequestEligible {
				t.Fatalf("expected an active and eligible Smart Hour offer: %#v", offer)
			}
			if offer.OutputAmount.StringFixed(2) != "100.00" || offer.DifferenceFromBNRInRON.StringFixed(2) != "0.00" {
				t.Fatalf("unexpected Smart Hour calculation: %#v", offer)
			}
		}
	}
	if !found {
		t.Fatal("expected active Smart Hour offer")
	}

	service.now = func() time.Time { return time.Date(2026, 8, 19, 8, 30, 0, 0, time.UTC) }
	outside, err := service.Compare(context.Background(), ComparisonRequest{From: "RON", To: "EUR", Amount: decimal.NewFromInt(500)})
	if err != nil {
		t.Fatal(err)
	}
	preview, found := findOffer(outside.Offers, "raiffeisen_smart_hour")
	if !found || preview.ActiveNow {
		t.Fatalf("expected an inactive Smart Hour preview outside 10:00-11:00, got %#v", preview)
	}
}

func TestRaiffeisenSmartHourRejectsAmountAboveDailyLimit(t *testing.T) {
	service := NewComparisonService(fakeRateStore{latest: []domain.RateSnapshot{snapshot("bnr", "5", "5")}})
	service.now = func() time.Time { return time.Date(2026, 8, 19, 7, 30, 0, 0, time.UTC) }
	response, err := service.Compare(context.Background(), ComparisonRequest{From: "RON", To: "EUR", Amount: decimal.NewFromInt(8000)})
	if err != nil {
		t.Fatal(err)
	}
	preview, found := findOffer(response.Offers, "raiffeisen_smart_hour")
	if !found || preview.RequestEligible {
		t.Fatalf("expected an ineligible Smart Hour preview above the daily limit, got %#v", preview)
	}
	for _, notice := range response.ProviderNotices {
		if notice.Provider == "raiffeisen_smart_hour" && notice.RequestEligible {
			t.Fatal("expected the Smart Hour notice to flag the amount as ineligible")
		}
	}
}

func TestTradevilleUsesSymmetricMaximum50PipSpreadAroundBNR(t *testing.T) {
	service := NewComparisonService(fakeRateStore{latest: []domain.RateSnapshot{snapshot("bnr", "5", "5")}})
	service.now = func() time.Time { return time.Date(2026, 8, 19, 10, 0, 0, 0, time.UTC) }

	buying, err := service.Compare(context.Background(), ComparisonRequest{From: "RON", To: "EUR", Amount: decimal.NewFromInt(1000)})
	if err != nil {
		t.Fatal(err)
	}
	buyOffer, found := findOffer(buying.Offers, "tradeville")
	if !found || buyOffer.EffectiveRate.StringFixed(4) != "5.0025" || !buyOffer.Indicative {
		t.Fatalf("unexpected TradeVille RON to EUR estimate: %#v", buyOffer)
	}

	selling, err := service.Compare(context.Background(), ComparisonRequest{From: "EUR", To: "RON", Amount: decimal.NewFromInt(100)})
	if err != nil {
		t.Fatal(err)
	}
	sellOffer, found := findOffer(selling.Offers, "tradeville")
	if !found || sellOffer.EffectiveRate.StringFixed(4) != "4.9975" || sellOffer.OutputAmount.StringFixed(2) != "499.75" {
		t.Fatalf("unexpected TradeVille EUR to RON estimate: %#v", sellOffer)
	}
}

func TestTradevilleDoesNotInventANumericQuoteForOtherCurrencies(t *testing.T) {
	service := NewComparisonService(fakeRateStore{latest: []domain.RateSnapshot{snapshot("bnr", "5", "5")}})
	response, err := service.Compare(context.Background(), ComparisonRequest{From: "RON", To: "USD", Amount: decimal.NewFromInt(1000)})
	if err != nil {
		t.Fatal(err)
	}
	if containsProvider(response.Offers, "tradeville") {
		t.Fatal("TradeVille must not have a derived public quote outside EUR/RON")
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
	offer, found := findOffer(response.Offers, "bcr")
	if !found || !offer.Stale {
		t.Fatal("expected stale offer")
	}
}
