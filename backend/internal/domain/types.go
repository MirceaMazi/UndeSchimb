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
	"bnr":                           "BNR",
	"banca_transilvania":            "Banca Transilvania",
	"bcr":                           "BCR",
	"brd":                           "BRD",
	"brd_you":                       "BRD — curs YOU",
	"ing":                           "ING",
	"ing_preferential":              "ING curs avantajos",
	"raiffeisen":                    "Raiffeisen",
	"raiffeisen_smart_hour":         "Raiffeisen Smart Hour",
	"cec":                           "CEC Bank",
	"xtb":                           "XTB",
	"tradeville":                    "TradeVille",
	"revolut":                       "Revolut",
	"tavex":                         "Tavex",
	"luxor_bucharest":               "Luxor București",
	"banca_transilvania_negotiated": "Banca Transilvania — curs negociat",
	"bcr_preferential":              "BCR — curs preferențial George",
	"cec_digital":                   "CEC Bank — curs digital",
}

const (
	CategoryBanks             = "banks"
	CategoryBrokers           = "brokers"
	CategoryPhysicalExchanges = "physical_exchanges"
)

var ProviderCategories = map[string]string{
	"banca_transilvania":            CategoryBanks,
	"bcr":                           CategoryBanks,
	"brd":                           CategoryBanks,
	"brd_you":                       CategoryBanks,
	"ing":                           CategoryBanks,
	"ing_preferential":              CategoryBanks,
	"raiffeisen":                    CategoryBanks,
	"raiffeisen_smart_hour":         CategoryBanks,
	"cec":                           CategoryBanks,
	"xtb":                           CategoryBrokers,
	"tradeville":                    CategoryBrokers,
	"revolut":                       CategoryBrokers,
	"tavex":                         CategoryPhysicalExchanges,
	"luxor_bucharest":               CategoryPhysicalExchanges,
	"banca_transilvania_negotiated": CategoryBanks,
	"bcr_preferential":              CategoryBanks,
	"cec_digital":                   CategoryBanks,
}

type RateSnapshot struct {
	Provider    string          `json:"provider"`
	Currency    string          `json:"currency"`
	BuyRate     decimal.Decimal `json:"buy_rate"`
	SellRate    decimal.Decimal `json:"sell_rate"`
	FeePercent  decimal.Decimal `json:"fee_percent"`
	SourceURL   string          `json:"source_url"`
	EffectiveAt time.Time       `json:"effective_at"`
	FetchedAt   time.Time       `json:"fetched_at"`
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
