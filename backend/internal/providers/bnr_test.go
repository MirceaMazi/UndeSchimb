package providers

import (
	"testing"
	"time"
)

func TestParseBNRXMLAppliesMultiplier(t *testing.T) {
	input := `<DataSet><Body><Cube date="2026-08-19"><Rate currency="EUR" multiplier="100">523.4500</Rate><Rate currency="USD">4.5000</Rate><Rate currency="GBP">6.0000</Rate><Rate currency="CHF">5.5000</Rate></Cube></Body></DataSet>`
	snapshots, err := ParseBNRXML([]byte(input), time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if got := snapshots[0].BuyRate.StringFixed(4); got != "5.2345" {
		t.Fatalf("expected multiplier-normalized EUR rate, got %s", got)
	}
}
