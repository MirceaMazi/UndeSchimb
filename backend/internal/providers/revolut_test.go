package providers

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRevolutProviderFetchesAndNormalizesPublicStandardQuotes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/quote" || request.URL.Query().Get("country") != "RO" || request.URL.Query().Get("isRecipientAmount") != "false" {
			t.Fatalf("unexpected Revolut request: %s", request.URL.String())
		}
		from := request.URL.Query().Get("fromCurrency")
		to := request.URL.Query().Get("toCurrency")
		rate := "5.2"
		if from == "RON" {
			rate = "0.19"
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(writer, `{"sender":{"amount":100000,"currency":"%s"},"recipient":{"amount":10000,"currency":"%s"},"rate":{"from":"%s","to":"%s","rate":%s,"timestamp":1787175900000},"plans":[{"id":"STANDARD","fees":{"total":{"amount":1000,"currency":"%s"}}}]}`, from, to, from, to, rate, from)
	}))
	defer server.Close()

	provider := &RevolutProvider{client: server.Client(), baseURL: server.URL}
	snapshots, err := provider.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshots) != 4 {
		t.Fatalf("expected four Revolut snapshots, got %d", len(snapshots))
	}
	wantBuy := "5.1485"
	wantSell := "5.3158"
	if snapshots[0].Provider != "revolut" || snapshots[0].BuyRate.StringFixed(4) != wantBuy || snapshots[0].SellRate.StringFixed(4) != wantSell {
		t.Fatalf("unexpected normalized Revolut snapshot: %#v", snapshots[0])
	}
	if snapshots[0].FeePercent.String() != "1" || snapshots[0].EffectiveAt != time.UnixMilli(1787175900000).UTC() {
		t.Fatalf("unexpected Revolut fee or timestamp: %#v", snapshots[0])
	}
}

func TestRevolutProviderRejectsIncompleteQuote(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"rate":{"rate":0}}`))
	}))
	defer server.Close()

	provider := &RevolutProvider{client: server.Client(), baseURL: server.URL}
	if _, err := provider.Fetch(context.Background()); err == nil {
		t.Fatal("expected an incomplete Revolut quote to fail")
	}
}
