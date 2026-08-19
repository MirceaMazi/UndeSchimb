package providers

import (
	"testing"
	"time"
)

func TestParseBancaTransilvaniaXML(t *testing.T) {
	payload := `<xml><updateDate name="2026-08-19 16:20:02"/><exchangeRates><currency name="USD"><sell><value>4.5924</value></sell><buy><value>4.4123</value></buy></currency><currency name="EUR"><sell><value>5.3050</value></sell><buy><value>5.1850</value></buy></currency><currency name="GBP"><sell><value>6.2524</value></sell><buy><value>6.0072</value></buy></currency><currency name="CHF"><sell><value>5.6520</value></sell><buy><value>5.5320</value></buy></currency></exchangeRates></xml>`
	snapshots, err := ParseBancaTransilvaniaXML([]byte(payload), time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if got := snapshots[1].SellRate.StringFixed(4); got != "5.3050" {
		t.Fatalf("unexpected EUR sell rate: %s", got)
	}
}
