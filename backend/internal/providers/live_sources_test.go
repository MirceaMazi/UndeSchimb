package providers

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"
)

// Opt in when diagnosing an upstream change; normal tests never need the network.
func TestPublicRateSourcesLive(t *testing.T) {
	if os.Getenv("UNDESCHIMB_LIVE_SOURCES") != "1" {
		t.Skip("set UNDESCHIMB_LIVE_SOURCES=1 to verify current CEC and XTB sources")
	}
	client := &http.Client{Timeout: 12 * time.Second}
	for _, provider := range []Provider{NewCECProvider(client), NewXTBProvider(client, nil)} {
		t.Run(provider.ID(), func(t *testing.T) {
			t.Parallel()
			// Use the same total deadline as the production collector.
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			rates, err := provider.Fetch(ctx)
			if err != nil {
				t.Fatal(err)
			}
			currencies := make(map[string]bool, 4)
			for _, rate := range rates {
				if err := rate.Validate(); err != nil {
					t.Fatal(err)
				}
				if rate.Provider != provider.ID() || rate.SourceURL == "" {
					t.Fatalf("unexpected provider or missing source for %s", rate.Currency)
				}
				currencies[rate.Currency] = true
			}
			for _, currency := range []string{"EUR", "USD", "GBP", "CHF"} {
				if !currencies[currency] {
					t.Errorf("missing %s quote", currency)
				}
			}
			t.Logf("%s: collected %d valid currency quotes", provider.ID(), len(rates))
		})
	}
}
