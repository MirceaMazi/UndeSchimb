package providers

import (
	"testing"
	"time"
)

func TestParseTavexRates(t *testing.T) {
	document := `<table><tbody>
	<tr role="row" class="list-table__row js-filter-search-row"><td><abbr>EUR</abbr><a>Euro</a></td><td class="list-table__col list-table__col--value">5.10</td><td class="list-table__col list-table__col--value">5.367</td></tr>
	<tr role="row" class="list-table__row js-filter-search-row"><td><abbr>USD</abbr><a>Dolar</a></td><td class="list-table__col list-table__col--value">4.335</td><td class="list-table__col list-table__col--value">4.561</td></tr>
	<tr role="row" class="list-table__row js-filter-search-row"><td><abbr>GBP</abbr><a>Liră</a></td><td class="list-table__col list-table__col--value">5.942</td><td class="list-table__col list-table__col--value">6.39</td></tr>
	<tr role="row" class="list-table__row js-filter-search-row"><td><abbr>CHF</abbr><a>Franc</a></td><td class="list-table__col list-table__col--value">5.352</td><td class="list-table__col list-table__col--value">5.748</td></tr>
	</tbody></table>`

	rates, err := ParseTavexRates(document, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if len(rates) != 4 {
		t.Fatalf("expected four Tavex rates, got %d", len(rates))
	}
	if rates[0].Provider != "tavex" || rates[0].BuyRate.String() != "5.1" || rates[0].SellRate.String() != "5.367" {
		t.Fatalf("unexpected Tavex EUR rate: %#v", rates[0])
	}
}

func TestParseTavexRatesRejectsMissingCurrency(t *testing.T) {
	document := `<tr class="list-table__row"><td><abbr>EUR</abbr></td><td class="list-table__col--value">5.10</td><td class="list-table__col--value">5.367</td></tr>`
	if _, err := ParseTavexRates(document, time.Now().UTC()); err == nil {
		t.Fatal("expected a missing-currency error")
	}
}
