package providers

import (
	"testing"
	"time"
)

func TestParseLuxorBucharestRates(t *testing.T) {
	document := `<table><tbody>
	<tr><td><strong>EUR</strong> - Euro</td><td>5.210 Lei</td><td>5.230 Lei</td></tr>
	<tr><td><strong>USD</strong> - Dolar</td><td>4.490 Lei</td><td>4.530 Lei</td></tr>
	<tr><td><strong>GBP</strong> - Liră</td><td>6.070 Lei</td><td>6.105 Lei</td></tr>
	<tr><td><strong>CHF</strong> - Franc</td><td>5.522 Lei</td><td>5.575 Lei</td></tr>
	</tbody></table>`
	snapshots, err := ParseLuxorBucharestRates(document, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshots) != 4 {
		t.Fatalf("expected four Luxor snapshots, got %d", len(snapshots))
	}
	if snapshots[0].Provider != "luxor_bucharest" || snapshots[0].BuyRate.String() != "5.21" || snapshots[0].SellRate.String() != "5.23" {
		t.Fatalf("unexpected Luxor EUR snapshot: %#v", snapshots[0])
	}
}

func TestPhysicalExchangeParsersRejectMissingCurrencies(t *testing.T) {
	if _, err := ParseLuxorBucharestRates(`<tr><td><strong>EUR</strong></td><td>5.20 Lei</td><td>5.30 Lei</td></tr>`, time.Now()); err == nil {
		t.Fatal("expected incomplete Luxor table to fail")
	}
}
