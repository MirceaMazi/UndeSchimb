package providers

import (
	"testing"
	"time"
)

func TestParseBCRRates(t *testing.T) {
	payload := []byte(`{"fx":[
		{"currency":"EUR","exchangeRate":{"lastModified":"2026-08-19T10:00:00Z","buy":5.16,"sell":5.32}},
		{"currency":"USD","exchangeRate":{"lastModified":"2026-08-19T10:00:00Z","buy":4.41,"sell":4.59}},
		{"currency":"GBP","exchangeRate":{"lastModified":"2026-08-19T10:00:00Z","buy":5.98,"sell":6.27}},
		{"currency":"CHF","exchangeRate":{"lastModified":"2026-08-19T10:00:00Z","buy":5.49,"sell":5.69}}
	]}`)
	rates, err := ParseBCRRates(payload, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if len(rates) != 4 || rates[0].SourceURL != BCRSourceURL {
		t.Fatalf("unexpected BCR rates: %#v", rates)
	}
}

func TestParseRaiffeisenRates(t *testing.T) {
	payload := []byte(`{"rates":[{"currencyList":[
		{"currencyPair":{"left":{"code":"EUR"}},"buyRate":{"value":5.17},"sellRate":{"value":5.32},"listCreationDateTime":"2026-08-19T10:00:00Z"},
		{"currencyPair":{"left":{"code":"USD"}},"buyRate":{"value":4.41},"sellRate":{"value":4.59},"listCreationDateTime":"2026-08-19T10:00:00Z"},
		{"currencyPair":{"left":{"code":"GBP"}},"buyRate":{"value":6.05},"sellRate":{"value":6.21},"listCreationDateTime":"2026-08-19T10:00:00Z"},
		{"currencyPair":{"left":{"code":"CHF"}},"buyRate":{"value":5.52},"sellRate":{"value":5.66},"listCreationDateTime":"2026-08-19T10:00:00Z"}
	]}]}`)
	rates, err := ParseRaiffeisenRates(payload, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if len(rates) != 4 || rates[3].SourceURL != RaiffeisenSourceURL {
		t.Fatalf("unexpected Raiffeisen rates: %#v", rates)
	}
}

func TestParseINGRatesUsesStandardInsteadOfPromotionalRate(t *testing.T) {
	document := `
fxRatesData.currencies = [
{"code":"EUR","sell":{"promo1":"5.20","promo2":"5.35"},"buy":{"promo1":"5.18","promo2":"5.14"}},
{"code":"USD","sell":{"promo1":"4.50","promo2":"4.60"},"buy":{"promo1":"4.48","promo2":"4.41"}},
{"code":"GBP","sell":{"promo1":"6.10","promo2":"6.25"},"buy":{"promo1":"6.08","promo2":"6.00"}},
{"code":"CHF","sell":{"promo1":"5.60","promo2":"5.70"},"buy":{"promo1":"5.56","promo2":"5.48"}}
];`
	rates, err := ParseINGRates(document, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if got := rates[0].SellRate.String(); got != "5.35" {
		t.Fatalf("expected standard sell rate 5.35, got %s", got)
	}
}

func TestParseCECRatesSkipsBNRReferenceColumn(t *testing.T) {
	document := `<table>
		<tr><td>EUR</td><td>Euro</td><td>5,2419</td><td>5,1956</td><td>5,2878</td></tr>
		<tr><td>USD</td><td>Dolar american</td><td>4,5195</td><td>4,4700</td><td>4,5872</td></tr>
		<tr><td>GBP</td><td>Lira sterlina</td><td>6,1284</td><td>6,0373</td><td>6,2184</td></tr>
		<tr><td>CHF</td><td>Franc elvetian</td><td>5,5830</td><td>5,5181</td><td>5,6390</td></tr>
	</table>`
	rates, err := ParseCECRates(document, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if got := rates[0].BuyRate.String(); got != "5.1956" {
		t.Fatalf("expected CEC buy rate 5.1956, got %s", got)
	}
}
