package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"github.com/undeschimb/undeschimb/internal/domain"
)

const staleAfter = 30 * time.Minute

type RateReader interface {
	LatestRates(context.Context, string) ([]domain.RateSnapshot, error)
	History(context.Context, string, string, time.Time) ([]domain.RateSnapshot, error)
}

type ComparisonService struct {
	rates         RateReader
	now           func() time.Time
	extraHolidays map[string]struct{}
}

func NewComparisonService(rates RateReader, holidays ...map[string]struct{}) *ComparisonService {
	extraHolidays := make(map[string]struct{})
	if len(holidays) > 0 && holidays[0] != nil {
		extraHolidays = holidays[0]
	}
	return &ComparisonService{rates: rates, now: time.Now, extraHolidays: extraHolidays}
}

type ComparisonRequest struct {
	From   string
	To     string
	Amount decimal.Decimal
}

type ComparisonResponse struct {
	From            string           `json:"from"`
	To              string           `json:"to"`
	InputAmount     decimal.Decimal  `json:"input_amount"`
	BNROutput       decimal.Decimal  `json:"bnr_output"`
	BNRRate         decimal.Decimal  `json:"bnr_rate"`
	BNRFetchedAt    time.Time        `json:"bnr_fetched_at"`
	Offers          []Offer          `json:"offers"`
	ProviderNotices []ProviderNotice `json:"provider_notices"`
	MissingSources  []string         `json:"missing_sources"`
}

type Offer struct {
	Provider               string          `json:"provider"`
	ProviderName           string          `json:"provider_name"`
	Category               string          `json:"category"`
	OfferType              string          `json:"offer_type"`
	EffectiveRate          decimal.Decimal `json:"effective_rate"`
	OutputAmount           decimal.Decimal `json:"output_amount"`
	DifferenceFromBNR      decimal.Decimal `json:"difference_from_bnr"`
	DifferenceFromBNRInRON decimal.Decimal `json:"difference_from_bnr_ron"`
	DifferencePercent      decimal.Decimal `json:"difference_percent"`
	FeePercent             decimal.Decimal `json:"fee_percent"`
	SourceURL              string          `json:"source_url"`
	FetchedAt              time.Time       `json:"fetched_at"`
	EffectiveAt            time.Time       `json:"effective_at"`
	Stale                  bool            `json:"stale"`
	Indicative             bool            `json:"indicative"`
	Conditional            bool            `json:"conditional"`
	Conditions             []string        `json:"conditions"`
	LocationPolicy         string          `json:"location_policy"`
	LocationLabel          string          `json:"location_label"`
	LocationNote           string          `json:"location_note"`
}

type ProviderNotice struct {
	Provider        string   `json:"provider"`
	ProviderName    string   `json:"provider_name"`
	Category        string   `json:"category"`
	Kind            string   `json:"kind"`
	Title           string   `json:"title"`
	Description     string   `json:"description"`
	SourceURL       string   `json:"source_url"`
	ActiveNow       bool     `json:"active_now"`
	RequestEligible bool     `json:"request_eligible"`
	Conditions      []string `json:"conditions"`
}

var comparableProviders = []string{
	"banca_transilvania", "bcr", "brd", "ing", "ing_preferential",
	"raiffeisen", "cec", "xtb", "revolut", "tavex", "luxor_bucharest",
}

func (s *ComparisonService) Compare(ctx context.Context, request ComparisonRequest) (ComparisonResponse, error) {
	request.From = domain.NormalizeCurrency(request.From)
	request.To = domain.NormalizeCurrency(request.To)
	if request.Amount.LessThanOrEqual(decimal.Zero) {
		return ComparisonResponse{}, fmt.Errorf("amount must be greater than zero")
	}
	if !((request.From == "RON" && request.To != "RON") || (request.To == "RON" && request.From != "RON")) {
		return ComparisonResponse{}, fmt.Errorf("only RON to/from EUR, USD, GBP, or CHF is supported")
	}
	currency := request.To
	if currency == "RON" {
		currency = request.From
	}
	snapshots, err := s.rates.LatestRates(ctx, currency)
	if err != nil {
		return ComparisonResponse{}, err
	}
	var bnr *domain.RateSnapshot
	byProvider := make(map[string]domain.RateSnapshot)
	for _, snapshot := range snapshots {
		if snapshot.Provider == "bnr" {
			copy := snapshot
			bnr = &copy
			continue
		}
		byProvider[snapshot.Provider] = snapshot
	}
	if bnr == nil {
		return ComparisonResponse{}, fmt.Errorf("BNR reference rate is not available for %s", currency)
	}
	eurBNR := bnr
	if currency != "EUR" {
		eurSnapshots, eurErr := s.rates.LatestRates(ctx, "EUR")
		if eurErr != nil {
			return ComparisonResponse{}, eurErr
		}
		eurBNR = nil
		for _, snapshot := range eurSnapshots {
			if snapshot.Provider == "bnr" {
				copy := snapshot
				eurBNR = &copy
				break
			}
		}
		if eurBNR == nil {
			return ComparisonResponse{}, fmt.Errorf("BNR reference rate is not available for EUR")
		}
	}
	luxorEligible, requestEURValue := physicalExchangeEligibility("luxor_bucharest", request, bnr, eurBNR)
	bnrOutput := calculateOutput(request.From, request.Amount, bnr)
	response := ComparisonResponse{
		From: request.From, To: request.To, InputAmount: request.Amount, BNROutput: bnrOutput,
		BNRRate: rateForDirection(request.From, bnr), BNRFetchedAt: bnr.FetchedAt,
		Offers:          make([]Offer, 0, len(comparableProviders)+1),
		ProviderNotices: make([]ProviderNotice, 0, 2),
		MissingSources:  make([]string, 0, len(comparableProviders)),
	}
	for _, provider := range comparableProviders {
		snapshot, found := byProvider[provider]
		if !found {
			response.MissingSources = append(response.MissingSources, provider)
			continue
		}
		if provider == "luxor_bucharest" && !luxorEligible {
			continue
		}
		output := calculateOutput(request.From, request.Amount, &snapshot)
		difference := output.Sub(bnrOutput)
		differenceInRON := difference
		if request.From == "RON" {
			differenceInRON = difference.Mul(bnr.SellRate)
		}
		percentage := decimal.Zero
		if !bnrOutput.IsZero() {
			percentage = difference.Div(bnrOutput).Mul(decimal.NewFromInt(100))
		}
		category, offerType, conditional, conditions := offerMetadata(provider)
		locationPolicy, locationLabel, locationNote := offerLocation(provider)
		response.Offers = append(response.Offers, Offer{
			Provider: provider, ProviderName: domain.ProviderNames[provider], Category: category, OfferType: offerType,
			EffectiveRate: rateForDirection(request.From, &snapshot),
			OutputAmount:  output, DifferenceFromBNR: difference, DifferenceFromBNRInRON: differenceInRON,
			DifferencePercent: percentage, FeePercent: snapshot.FeePercent, SourceURL: snapshot.SourceURL,
			FetchedAt: snapshot.FetchedAt, EffectiveAt: snapshot.EffectiveAt,
			Stale: s.now().UTC().Sub(snapshot.FetchedAt) > staleAfter, Indicative: provider == "xtb" || provider == "revolut",
			Conditional: conditional, Conditions: conditions, LocationPolicy: locationPolicy,
			LocationLabel: locationLabel, LocationNote: locationNote,
		})
	}
	response.ProviderNotices = append(response.ProviderNotices,
		ingPreferentialNotice(), tradevilleNotice(currency), revolutNotice(),
		tavexNotice(request, bnr), luxorNotice(luxorEligible, requestEURValue),
	)
	if currency == "EUR" {
		notice, specialOffer := s.raiffeisenSmartHour(request, bnr, bnrOutput)
		response.ProviderNotices = append(response.ProviderNotices, notice)
		if specialOffer != nil {
			response.Offers = append(response.Offers, *specialOffer)
		}
	}
	sort.Slice(response.Offers, func(i, j int) bool {
		return response.Offers[i].OutputAmount.GreaterThan(response.Offers[j].OutputAmount)
	})
	return response, nil
}

func offerMetadata(provider string) (category, offerType string, conditional bool, conditions []string) {
	category = domain.ProviderCategories[provider]
	offerType = "standard"
	switch provider {
	case "ing_preferential":
		return category, "preferential", true, []string{
			"ING Go cu venit recurent și ING More: în limita a 10.000 RON pe lună",
			"ING Extra: curs avantajos indiferent de sumă",
			"Pentru sume mari poate exista o cotație personalizată în Home'Bank",
		}
	case "xtb":
		return category, "indicative", true, []string{
			"Cotația executabilă din platformă se poate actualiza mai repede",
			"Calculul include taxa de conversie publicată de XTB",
		}
	case "revolut":
		return category, "indicative", true, []string{
			"Cotația publică pentru planul Standard este dinamică și poate depinde de sumă",
			"Estimarea presupune că mai ai disponibilă limita lunară fără comision",
			"Comisionul exact pentru plan, limită și weekend trebuie confirmat în aplicație",
		}
	case "tavex":
		return category, "cash", true, []string{
			"Schimb disponibil numai cu numerar, pentru bancnote aflate în circulație",
			"Cursul publicat este identic în toate birourile Tavex din România",
			"Pentru peste 20.000 RON poți solicita un curs preferențial",
		}
	case "luxor_bucharest":
		return category, "cash", true, []string{
			"Curs pentru locația Piața Universității din București; Luxor publică alte valori pentru Arad",
			"Cursul afișat este valabil pentru peste 500 EUR sau echivalent, în limita disponibilității",
			"Cursul poate fi negociat direct la casa de schimb",
		}
	default:
		return category, offerType, false, []string{}
	}
}

func offerLocation(provider string) (policy, label, note string) {
	switch provider {
	case "tavex":
		return "same_all_locations", "București · birourile Tavex", "Același curs publicat și în celelalte locații Tavex"
	case "luxor_bucharest":
		return "varies_by_city", "București · Piața Universității", "Luxor publică un curs diferit pentru Arad"
	default:
		return "not_applicable", "", ""
	}
}

func physicalExchangeEligibility(provider string, request ComparisonRequest, selectedBNR, eurBNR *domain.RateSnapshot) (bool, decimal.Decimal) {
	ronEquivalent := request.Amount
	if request.From != "RON" {
		ronEquivalent = request.Amount.Mul(selectedBNR.BuyRate)
	}
	eurEquivalent := ronEquivalent.Div(eurBNR.BuyRate)
	if provider == "luxor_bucharest" {
		return eurEquivalent.GreaterThan(decimal.NewFromInt(500)), eurEquivalent
	}
	return true, eurEquivalent
}

func tavexNotice(request ComparisonRequest, selectedBNR *domain.RateSnapshot) ProviderNotice {
	ronEquivalent := request.Amount
	if request.From != "RON" {
		ronEquivalent = request.Amount.Mul(selectedBNR.BuyRate)
	}
	conditions := []string{
		"Cursul standard publicat nu are prag minim și este identic în toate birourile Tavex",
		"Schimbul se face numai cu numerar și cu unități întregi; diferența se restituie în RON",
		"Pentru sume mari, disponibilitatea valutei trebuie confirmată în prealabil",
	}
	if ronEquivalent.GreaterThan(decimal.NewFromInt(20000)) {
		conditions = append(conditions, "Suma ta depășește 20.000 RON: poți solicita un curs preferențial, păstrat timp de o oră")
	} else {
		conditions = append(conditions, "Peste echivalentul a 20.000 RON poți solicita separat un curs preferențial")
	}
	return ProviderNotice{
		Provider: "tavex", ProviderName: domain.ProviderNames["tavex"], Category: domain.CategoryPhysicalExchanges,
		Kind: "conditional", Title: "Curs standard fără prag minim", RequestEligible: true, ActiveNow: true,
		Description: "Tavex rămâne în clasament la cursul standard pentru orice sumă. O eventuală cotație preferențială nu înlocuiește automat cursul public.",
		SourceURL:   "https://tavex.ro/schimb-valutar/", Conditions: conditions,
	}
}

func luxorNotice(eligible bool, requestEURValue decimal.Decimal) ProviderNotice {
	kind := "conditional"
	title := "Curs disponibil peste 500 EUR"
	description := fmt.Sprintf("Suma introdusă valorează aproximativ %s EUR. Cursul Luxor București intră în clasament numai peste pragul publicat.", requestEURValue.StringFixed(2))
	if !eligible {
		kind = "ineligible"
		title = "Pragul de 500 EUR nu este îndeplinit"
	}
	return ProviderNotice{
		Provider: "luxor_bucharest", ProviderName: domain.ProviderNames["luxor_bucharest"], Category: domain.CategoryPhysicalExchanges,
		Kind: kind, Title: title, RequestEligible: eligible, ActiveNow: eligible,
		Description: description, SourceURL: "https://www.luxor-exchange.ro/bucuresti",
		Conditions: []string{
			"Cursul este valabil numai pentru sume mai mari de 500 EUR sau echivalent",
			"Oferta este pentru Piața Universității, București, în limita disponibilității",
			"Luxor publică un curs diferit pentru locația din Arad",
		},
	}
}

func tradevilleNotice(currency string) ProviderNotice {
	description := "TradeVille nu publică o cotație numerică înainte de autentificare. Pentru EUR–RON, conversiile din intervalul 09:00–16:00 folosesc cursul unei bănci partenere cu spread sub 50 pips."
	conditions := []string{
		"Cotația exactă trebuie verificată în platforma TradeVille",
		"TradeVille declară că nu percepe propriul comision de conversie",
	}
	if currency != "EUR" {
		description = "Pentru alte valute decât EUR, TradeVille folosește bănci corespondente; acestea pot aplica propriile comisioane și nu există o cotație publică de comparat."
		conditions = []string{"Cotația și eventualele comisioane trebuie verificate în platformă"}
	}
	return ProviderNotice{
		Provider: "tradeville", ProviderName: domain.ProviderNames["tradeville"], Category: domain.CategoryBrokers,
		Kind: "quote_required", Title: "Cotație disponibilă în platformă", Description: description,
		SourceURL: "https://tradeville.ro/costuri", ActiveNow: false, RequestEligible: true, Conditions: conditions,
	}
}

func revolutNotice() ProviderNotice {
	return ProviderNotice{
		Provider: "revolut", ProviderName: domain.ProviderNames["revolut"], Category: domain.CategoryBrokers,
		Kind: "conditional", Title: "Cotație publică pentru planul Standard",
		Description: "Colectăm cursul numeric din convertorul public Revolut și îl marcăm ca indicativ. Costul final poate depinde de sumă, plan, limita lunară rămasă și momentul schimbului.",
		SourceURL:   "https://www.revolut.com/ro-RO/legal/standard-fees/", ActiveNow: false, RequestEligible: true,
		Conditions: []string{
			"Standard: 1% peste limita lunară de 5.000 RON",
			"În weekend se poate aplica un comision de 1% pentru planul Standard",
			"Cursul și costul final trebuie confirmate în aplicație înainte de schimb",
		},
	}
}

func ingPreferentialNotice() ProviderNotice {
	return ProviderNotice{
		Provider: "ing_preferential", ProviderName: domain.ProviderNames["ing_preferential"], Category: domain.CategoryBanks,
		Kind: "conditional", Title: "Curs avantajos pentru pachete eligibile",
		Description: "ING publică un curs avantajos separat de cursul standard. Valoarea poate fi calculată, dar beneficiul depinde de pachetul și limitele clientului.",
		SourceURL:   "https://ing.ro/persoane-fizice/curs-valutar", ActiveNow: false, RequestEligible: true,
		Conditions: []string{
			"ING Go cu venit recurent și ING More: în limita a 10.000 RON pe lună",
			"ING Extra: curs avantajos indiferent de sumă",
			"Pentru sume mari poate exista o ofertă personalizată în Home'Bank",
		},
	}
}

func (s *ComparisonService) raiffeisenSmartHour(request ComparisonRequest, bnr *domain.RateSnapshot, bnrOutput decimal.Decimal) (ProviderNotice, *Offer) {
	foreignAmount := request.Amount
	if request.From == "RON" {
		foreignAmount = bnrOutput
	}
	requestEligible := !foreignAmount.GreaterThan(decimal.NewFromInt(1500))
	now := s.now().In(domain.BucharestLocation())
	activeNow := !domain.IsRomanianNonWorkingDay(now, s.extraHolidays) && now.Hour() == 10
	conditions := []string{
		"Doar EUR ↔ RON, în Smart Mobile, în zile lucrătoare între 10:00 și 11:00",
		"Maximum 1.500 EUR pe zi și 10.000 EUR cumulat pe lună",
		"Necesită cont Raiffeisen și limită lunară disponibilă",
	}
	notice := ProviderNotice{
		Provider: "raiffeisen_smart_hour", ProviderName: domain.ProviderNames["raiffeisen_smart_hour"],
		Category: domain.CategoryBanks, Kind: "scheduled", Title: "Schimb EUR la cursul BNR",
		Description: "Smart Hour aplică reperul BNR pentru schimburi eligibile EUR–RON. Oferta numerică intră în clasament numai cât timp intervalul este activ.",
		SourceURL:   "https://www.raiffeisen.ro/ro/persoane-fizice/produsele-noastre/digital-banking/mobile-banking.html",
		ActiveNow:   activeNow, RequestEligible: requestEligible, Conditions: conditions,
	}
	if !activeNow || !requestEligible {
		return notice, nil
	}
	return notice, &Offer{
		Provider: "raiffeisen_smart_hour", ProviderName: domain.ProviderNames["raiffeisen_smart_hour"],
		Category: domain.CategoryBanks, OfferType: "special", EffectiveRate: rateForDirection(request.From, bnr),
		OutputAmount: bnrOutput, DifferenceFromBNR: decimal.Zero, DifferenceFromBNRInRON: decimal.Zero,
		DifferencePercent: decimal.Zero, FeePercent: decimal.Zero, SourceURL: notice.SourceURL,
		FetchedAt: bnr.FetchedAt, EffectiveAt: bnr.EffectiveAt,
		Stale: s.now().UTC().Sub(bnr.FetchedAt) > staleAfter, Indicative: false,
		Conditional: true, Conditions: conditions,
	}
}

func calculateOutput(from string, amount decimal.Decimal, snapshot *domain.RateSnapshot) decimal.Decimal {
	if from == "RON" {
		return amount.Div(snapshot.SellRate)
	}
	return amount.Mul(snapshot.BuyRate)
}

func rateForDirection(from string, snapshot *domain.RateSnapshot) decimal.Decimal {
	if from == "RON" {
		return snapshot.SellRate
	}
	return snapshot.BuyRate
}

type HistoryPoint struct {
	Date         string          `json:"date"`
	ProviderRate decimal.Decimal `json:"provider_rate"`
	BNRRate      decimal.Decimal `json:"bnr_rate"`
}

type HistoryResponse struct {
	Provider string         `json:"provider"`
	Currency string         `json:"currency"`
	Side     string         `json:"side"`
	Points   []HistoryPoint `json:"points"`
}

func (s *ComparisonService) History(ctx context.Context, provider, currency, side string, days int) (HistoryResponse, error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	currency = domain.NormalizeCurrency(currency)
	side = strings.ToLower(strings.TrimSpace(side))
	if _, exists := domain.ProviderNames[provider]; !exists || provider == "bnr" {
		return HistoryResponse{}, fmt.Errorf("unknown provider")
	}
	if currency == "RON" || !domain.ValidCurrency(currency) {
		return HistoryResponse{}, fmt.Errorf("unsupported currency")
	}
	if side != "buy" && side != "sell" {
		return HistoryResponse{}, fmt.Errorf("side must be buy or sell")
	}
	if days != 7 && days != 30 && days != 90 {
		return HistoryResponse{}, fmt.Errorf("period must be 7, 30, or 90 days")
	}
	since := s.now().UTC().AddDate(0, 0, -days)
	providerSnapshots, err := s.rates.History(ctx, provider, currency, since)
	if err != nil {
		return HistoryResponse{}, err
	}
	bnrSnapshots, err := s.rates.History(ctx, "bnr", currency, since)
	if err != nil {
		return HistoryResponse{}, err
	}
	providerByDate := latestByDate(providerSnapshots, side)
	bnrByDate := latestByDate(bnrSnapshots, side)
	points := make([]HistoryPoint, 0, len(providerByDate))
	for date, providerRate := range providerByDate {
		if bnrRate, exists := bnrByDate[date]; exists {
			points = append(points, HistoryPoint{Date: date, ProviderRate: providerRate, BNRRate: bnrRate})
		}
	}
	sort.Slice(points, func(i, j int) bool { return points[i].Date < points[j].Date })
	return HistoryResponse{Provider: provider, Currency: currency, Side: side, Points: points}, nil
}

func latestByDate(snapshots []domain.RateSnapshot, side string) map[string]decimal.Decimal {
	result := make(map[string]decimal.Decimal)
	for _, snapshot := range snapshots {
		date := snapshot.FetchedAt.In(bucharest()).Format("2006-01-02")
		result[date] = snapshot.BuyRate
		if side == "sell" {
			result[date] = snapshot.SellRate
		}
	}
	return result
}

func bucharest() *time.Location {
	location, err := time.LoadLocation("Europe/Bucharest")
	if err != nil {
		return time.UTC
	}
	return location
}
