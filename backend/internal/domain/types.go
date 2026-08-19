package domain

import (
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

var SupportedCurrencies = map[string]struct{}{
	"EUR": {}, "USD": {}, "GBP": {}, "CHF": {}, "RON": {},
}

var ProviderNames = map[string]string{
	"bnr":         "BNR",
	"banca_transilvania": "Banca Transilvania",
	"bcr":         "BCR",
	"brd":         "BRD",
	"ing":         "ING",
	"raiffeisen":  "Raiffeisen",
	"cec":         "CEC Bank",
	"xtb":         "XTB",
}

type RateSnapshot struct {
	Provider     string          `json:"provider"`
	Currency     string          `json:"currency"`
	BuyRate      decimal.Decimal `json:"buy_rate"`
	SellRate     decimal.Decimal `json:"sell_rate"`
	FeePercent   decimal.Decimal `json:"fee_percent"`
	SourceURL    string          `json:"source_url"`
	EffectiveAt  time.Time       `json:"effective_at"`
	FetchedAt    time.Time       `json:"fetched_at"`
}

func (s RateSnapshot) Validate() error {
	if _, ok := ProviderNames[s.Provider]; !ok {
		return fmt.Errorf("unknown provider %q", s.Provider)
	}
	if s.Currency == "RON" {
		return fmt.Errorf("RON cannot be stored as a quoted currency")
	}
	if _, ok := SupportedCurrencies[s.Currency]; !ok {
		return fmt.Errorf("unsupported currency %q", s.Currency)
	}
	if !s.BuyRate.IsPositive() || !s.SellRate.IsPositive() {
		return fmt.Errorf("rates must be positive")
	}
	if s.SourceURL == "" {
		return fmt.Errorf("source URL is required")
	}
	return nil
}

func NormalizeCurrency(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}

func ValidCurrency(value string) bool {
	_, ok := SupportedCurrencies[NormalizeCurrency(value)]
	return ok
}

