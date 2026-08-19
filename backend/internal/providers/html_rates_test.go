package providers

import "testing"

func TestExtractAccountRates(t *testing.T) {
	document := `<section><h2>Schimb valutar în cont</h2><table>
<tr><th>Monedă</th><th>Cumpărare</th><th>Vânzare</th></tr>
<tr><td>EUR</td><td>5,1010</td><td>5,3020</td></tr>
<tr><td>USD</td><td>4,4000</td><td>4,6000</td></tr>
<tr><td>GBP</td><td>5,9000</td><td>6,1000</td></tr>
<tr><td>CHF</td><td>5,3000</td><td>5,5000</td></tr>
</table></section>`
	quotes, err := ExtractAccountRates(document, []string{"Schimb valutar în cont"})
	if err != nil {
		t.Fatal(err)
	}
	if got := quotes["EUR"].Sell.StringFixed(4); got != "5.3020" {
		t.Fatalf("unexpected EUR sell rate: %s", got)
	}
}

func TestExtractAccountRatesRejectsMissingCurrency(t *testing.T) {
	_, err := ExtractAccountRates("Schimb valutar în cont EUR 5.1000 5.3000", []string{"Schimb valutar"})
	if err == nil {
		t.Fatal("expected a missing-currency error")
	}
}

func TestExtractAccountRatesReadsColumnarAccountTable(t *testing.T) {
	document := `<div id="tabAccountExchangeRates"><div><p class="heading">Cod valută</p><p>EUR</p><p>USD</p><p>GBP</p><p>CHF</p></div><div><p class="heading">Cump. BRD (RON)</p><p>5.1000</p><p>4.4000</p><p>6.0000</p><p>5.3000</p></div><div><p class="heading">Vânz. BRD (RON)</p><p>5.3000</p><p>4.6000</p><p>6.2000</p><p>5.5000</p></div></div><div id="tabExchangeYou">`
	quotes, err := ExtractAccountRates(document, []string{"Schimb valutar în cont"})
	if err != nil {
		t.Fatal(err)
	}
	if got := quotes["CHF"].Buy.StringFixed(4); got != "5.3000" {
		t.Fatalf("unexpected CHF buy rate: %s", got)
	}
}
