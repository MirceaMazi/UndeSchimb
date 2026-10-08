package providers

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

func TestExtractXTBPairMeta(t *testing.T) {
	document := `window.websocketClientData = {'url': 'wss://api5widget.x-station.eu/widget', 'endpoint': 'meta1', 'user': '10383757', 'accountId': 'meta1_10383757', 'accessCode': 'access'}; const instrument = {'symbolXapi5': '1_EURRON_5'};`
	meta, err := ExtractXTBPairMeta(document, "https://www.xtb.com/ro/forex/eur-ron")
	if err != nil {
		t.Fatal(err)
	}
	if meta.Key != "1_EURRON_5" || meta.Connection.AccountID != "meta1_10383757" {
		t.Fatalf("unexpected widget metadata: %#v", meta)
	}
}

func TestExtractXTBPairMetaCurrentInstrumentWidget(t *testing.T) {
	document, err := os.ReadFile("testdata/xtb-instrument-widget.html")
	if err != nil {
		t.Fatal(err)
	}
	for name, markup := range map[string]string{
		"current unquoted symbol":  string(document),
		"quoted symbol":            strings.Replace(string(document), "symbolXapi5:", `"symbolXapi5":`, 1),
		"unquoted connection keys": strings.NewReplacer("'url':", "url:", "'endpoint':", "endpoint:", "'user':", "user:", "'accountId':", "accountId:", "'accessCode':", "accessCode:").Replace(string(document)),
	} {
		t.Run(name, func(t *testing.T) {
			meta, err := ExtractXTBPairMeta(markup, xtbBaseURL+"eur-ron")
			if err != nil {
				t.Fatal(err)
			}
			if meta.Key != "1_EURRON_5" || meta.Connection.AccountID != "meta1_widget-user" || meta.URL != xtbBaseURL+"eur-ron" {
				t.Fatalf("unexpected metadata: %#v", meta)
			}
		})
	}
}

func TestExtractXTBPairMetaRejectsIncompleteOrLookalikeFields(t *testing.T) {
	document, err := os.ReadFile("testdata/xtb-instrument-widget.html")
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"symbolXapi5", "url", "endpoint", "user", "accountId", "accessCode"} {
		t.Run(field, func(t *testing.T) {
			markup := strings.ReplaceAll(string(document), field, "previous_"+field)
			if _, err := ExtractXTBPairMeta(markup, xtbBaseURL+"eur-ron"); err == nil {
				t.Fatal("expected missing field to reject metadata")
			}
		})
	}
}

func TestQuoteFromMessage(t *testing.T) {
	var message xtbSocketMessage
	payload := `{"reqId":"getAndSubscribeElement","response":[{"element":{"elements":[{"value":{"xcfdtick":{"key":"1_EURRON_5","bid":5.0812,"ask":5.0821}}}]}}]}`
	if err := json.Unmarshal([]byte(payload), &message); err != nil {
		t.Fatal(err)
	}
	bid, ask, err := quoteFromMessage(message, "1_EURRON_5")
	if err != nil {
		t.Fatal(err)
	}
	if bid.String() != "5.0812" || ask.String() != "5.0821" {
		t.Fatalf("unexpected quote %s/%s", bid, ask)
	}
}

func TestXTBWeekendFee(t *testing.T) {
	weekend := time.Date(2026, time.August, 22, 10, 0, 0, 0, bucharestLocation())
	if got := xtbFee(weekend, nil).String(); got != "0.008" {
		t.Fatalf("expected weekend fee, got %s", got)
	}
	weekday := time.Date(2026, time.August, 19, 10, 0, 0, 0, bucharestLocation())
	if got := xtbFee(weekday, nil).String(); got != "0.005" {
		t.Fatalf("expected weekday fee, got %s", got)
	}
}
