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
	ActiveNow              bool            `json:"active_now"`
	RequestEligible        bool            `json:"request_eligible"`
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
	"banca_transilvania", "bcr", "brd", "brd_you", "ing", "ing_preferential",
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
		if provider == "brd_you" && currency != "EUR" {
			continue
		}
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
			Conditional: conditional, ActiveNow: true, RequestEligible: true,
			Conditions: conditions, LocationPolicy: locationPolicy,
			LocationLabel: locationLabel, LocationNote: locationNote,
		})
	}
	tradevilleNotice, tradevilleOffer := s.tradevilleEstimate(request, currency, bnr, bnrOutput)
	response.ProviderNotices = append(response.ProviderNotices,
		ingPreferentialNotice(), brdYouNotice(currency), tradevilleNotice, revolutNotice(),
		bancaTransilvaniaNegotiatedNotice(currency, request, bnr, eurBNR),
		bcrPreferentialNotice(currency), cecDigitalNotice(currency),
		tavexNotice(request, bnr), luxorNotice(luxorEligible, requestEURValue),
	)
	if tradevilleOffer != nil {
		response.Offers = append(response.Offers, *tradevilleOffer)
	}
	if currency == "EUR" {
		notice, specialOffer := s.raiffeisenSmartHour(request, bnr, bnrOutput)
		response.ProviderNotices = append(response.ProviderNotices, notice)
		response.Offers = append(response.Offers, *specialOffer)
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
			"Peste 20.000 EUR și peste 50.000 EUR există niveluri mai bune în Home'Bank, indiferent de pachet",
		}
	case "brd_you":
		return category, "preferential", true, []string{
			"Curs preferențial EUR ↔ RON publicat separat pentru aplicația YOU BRD",
			"Disponibil 24/7 pentru utilizatorii YOU BRD",
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

func (s *ComparisonService) tradevilleEstimate(request ComparisonRequest, currency string, bnr *domain.RateSnapshot, bnrOutput decimal.Decimal) (ProviderNotice, *Offer) {
	sourceURL := "https://tradeville.ro/costuri"
	if currency != "EUR" {
		return ProviderNotice{
			Provider: "tradeville", ProviderName: domain.ProviderNames["tradeville"], Category: domain.CategoryBrokers,
			Kind: "quote_required", Title: "Cotație disponibilă în platformă",
			Description: "Pentru alte valute decât EUR, TradeVille folosește bănci corespondente; acestea pot aplica propriile comisioane și nu există o cotație publică de comparat.",
			SourceURL:   sourceURL, ActiveNow: false, RequestEligible: false,
			Conditions: []string{"Cotația și eventualele comisioane trebuie verificate în platformă"},
		}, nil
	}

	// TradeVille publishes a maximum total EUR/RON spread of 50 pips, but not
	// the executable bid and ask. For a transparent estimate, use BNR as the
	// assumed midpoint and distribute the maximum spread symmetrically.
	halfSpread := decimal.RequireFromString("0.0025")
	effectiveRate := bnr.BuyRate.Sub(halfSpread)
	if request.From == "RON" {
		effectiveRate = bnr.SellRate.Add(halfSpread)
	}
	output := request.Amount.Mul(effectiveRate)
	if request.From == "RON" {
		output = request.Amount.Div(effectiveRate)
	}
	difference := output.Sub(bnrOutput)
	differenceInRON := difference
	if request.From == "RON" {
		differenceInRON = difference.Mul(bnr.SellRate)
	}
	percentage := decimal.Zero
	if !bnrOutput.IsZero() {
		percentage = difference.Div(bnrOutput).Mul(decimal.NewFromInt(100))
	}
	now := s.now().In(domain.BucharestLocation())
	activeNow := now.Hour() >= 9 && now.Hour() < 16 && !domain.IsRomanianNonWorkingDay(now, s.extraHolidays)
	conditions := []string{
		"Doar EUR ↔ RON, pentru schimburile solicitate în intervalul 09:00–16:00",
		"TradeVille publică un spread total sub 50 pips prin băncile partenere, nu un bid/ask live",
		"Estimarea presupune BNR ca punct median și aplică 25 pips pe fiecare sens",
		"Cotația executabilă trebuie confirmată în platforma TradeVille",
	}
	notice := ProviderNotice{
		Provider: "tradeville", ProviderName: domain.ProviderNames["tradeville"], Category: domain.CategoryBrokers,
		Kind: "conditional", Title: "Estimare BNR cu spread maxim de 50 pips",
		Description: "Estimăm cotația TradeVille în jurul cursului BNR deoarece brokerul publică numai limita spread-ului. Rezultatul real poate fi diferit.",
		SourceURL:   sourceURL, ActiveNow: activeNow, RequestEligible: true, Conditions: conditions,
	}
	offer := &Offer{
		Provider: "tradeville", ProviderName: domain.ProviderNames["tradeville"], Category: domain.CategoryBrokers,
		OfferType: "indicative", EffectiveRate: effectiveRate, OutputAmount: output,
		DifferenceFromBNR: difference, DifferenceFromBNRInRON: differenceInRON, DifferencePercent: percentage,
		FeePercent: decimal.Zero, SourceURL: sourceURL, FetchedAt: bnr.FetchedAt, EffectiveAt: bnr.EffectiveAt,
		Stale: s.now().UTC().Sub(bnr.FetchedAt) > staleAfter, Indicative: true, Conditional: true,
		ActiveNow: activeNow, RequestEligible: true, Conditions: conditions,
		LocationPolicy: "not_applicable",
	}
	return notice, offer
}

func brdYouNotice(currency string) ProviderNotice {
	eligible := currency == "EUR"
	return ProviderNotice{
		Provider: "brd_you", ProviderName: domain.ProviderNames["brd_you"], Category: domain.CategoryBanks,
		Kind: "conditional", Title: "Curs preferențial EUR–RON în YOU BRD",
		Description: "BRD publică separat cursul pentru schimburile din aplicația YOU. Pentru EUR–RON îl colectăm și îl poți include în clasament din comutatorul de oferte speciale.",
		SourceURL:   "https://www.brd.ro/curs-valutar-si-dobanzi-de-referinta", ActiveNow: eligible, RequestEligible: eligible,
		Conditions: []string{"Disponibil 24/7 în YOU BRD", "Oferta publicată separat este pentru EUR ↔ RON"},
	}
}

func bancaTransilvaniaNegotiatedNotice(currency string, request ComparisonRequest, selectedBNR, eurBNR *domain.RateSnapshot) ProviderNotice {
	foreignAmount := request.Amount
	if request.From == "RON" {
		foreignAmount = calculateOutput(request.From, request.Amount, selectedBNR)
	}
	eligible := false
	threshold := "peste 100.000 de unități pentru EUR/USD/GBP ↔ RON"
	if currency == "EUR" || currency == "USD" || currency == "GBP" {
		eligible = foreignAmount.GreaterThan(decimal.NewFromInt(100000))
	} else {
		ronEquivalent := request.Amount
		if request.From != "RON" {
			ronEquivalent = request.Amount.Mul(selectedBNR.BuyRate)
		}
		eurEquivalent := ronEquivalent.Div(eurBNR.BuyRate)
		eligible = eurEquivalent.GreaterThan(decimal.NewFromInt(25000))
		threshold = "peste echivalentul a 25.000 EUR pentru celelalte perechi"
	}
	return ProviderNotice{
		Provider: "banca_transilvania_negotiated", ProviderName: domain.ProviderNames["banca_transilvania_negotiated"], Category: domain.CategoryBanks,
		Kind: "conditional", Title: "Cotație negociată în BT Pay",
		Description: "BT poate afișa o ofertă negociată separată pentru tranzacțiile mari. Cotația nu este publică, deci păstrăm cursul standard în clasament.",
		SourceURL:   "https://www.bancatransilvania.ro/wallet-bt-pay/termeni-si-conditii-ro", ActiveNow: false, RequestEligible: eligible,
		Conditions: []string{threshold, "Oferta exactă apare în BT Pay și poate fi acceptată sau refuzată", "Negocierea este disponibilă în zile bancare lucrătoare, între 09:00 și 17:30"},
	}
}

func bcrPreferentialNotice(currency string) ProviderNotice {
	eligible := currency == "EUR"
	return ProviderNotice{
		Provider: "bcr_preferential", ProviderName: domain.ProviderNames["bcr_preferential"], Category: domain.CategoryBanks,
		Kind: "conditional", Title: "Curs preferențial EUR în George",
		Description: "Programul de beneficii BCR include un curs preferențial EUR între conturile proprii. Valoarea exactă este afișată în George și nu poate fi calculată public.",
		SourceURL:   "https://www.bcr.ro/content/dam/ro/bcr/www_bcr_ro/Campanii/2025/regulamente/Regulamentul-Programului-de-Beneficii.pdf", ActiveNow: false, RequestEligible: eligible,
		Conditions: []string{"Nivel Advanced: până la 500 EUR pe lună", "Nivel Pro: până la 1.000 EUR pe lună", "Nivel Max/Max Invest: până la 2.000 EUR pe lună", "Nivelul și cotația trebuie confirmate în George"},
	}
}

func cecDigitalNotice(currency string) ProviderNotice {
	return ProviderNotice{
		Provider: "cec_digital", ProviderName: domain.ProviderNames["cec_digital"], Category: domain.CategoryBanks,
		Kind: "conditional", Title: "Cursul digital este deja inclus",
		Description: "CEC publică pentru canalele la distanță un curs cu 0,0025 RON mai avantajos decât cursul de cont de la ghișeu. UndeSchimb colectează direct această tabelă online, deci avantajul este deja reflectat în rândul CEC Bank.",
		SourceURL:   "https://cloud.cec.ro/presa/cec-bank-curs-de-schimb-preferential-pentru-schimburile-valutare-prin-operatiuni-la-distanta", ActiveNow: true, RequestEligible: currency == "EUR" || currency == "USD" || currency == "GBP" || currency == "CHF",
		Conditions: []string{"Valabil pentru EUR, USD, GBP și CHF ↔ RON", "Se aplică prin Internet Banking, Mobile Banking și Phone Banking"},
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
			"Peste 20.000 EUR și peste 50.000 EUR există niveluri mai bune în Home'Bank, indiferent de pachet",
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
	return notice, &Offer{
		Provider: "raiffeisen_smart_hour", ProviderName: domain.ProviderNames["raiffeisen_smart_hour"],
		Category: domain.CategoryBanks, OfferType: "special", EffectiveRate: rateForDirection(request.From, bnr),
		OutputAmount: bnrOutput, DifferenceFromBNR: decimal.Zero, DifferenceFromBNRInRON: decimal.Zero,
		DifferencePercent: decimal.Zero, FeePercent: decimal.Zero, SourceURL: notice.SourceURL,
		FetchedAt: bnr.FetchedAt, EffectiveAt: bnr.EffectiveAt,
		Stale: s.now().UTC().Sub(bnr.FetchedAt) > staleAfter, Indicative: false,
		Conditional: true, ActiveNow: activeNow, RequestEligible: requestEligible, Conditions: conditions,
		LocationPolicy: "not_applicable",
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
	Date         string           `json:"date"`
	ProviderRate decimal.Decimal  `json:"provider_rate"`
	BNRRate      *decimal.Decimal `json:"bnr_rate"`
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
		// A failed BNR collection must not hide valid provider observations.
		// Keep missing benchmarks explicit; never substitute zero or an older rate.
		point := HistoryPoint{Date: date, ProviderRate: providerRate}
		if bnrRate, exists := bnrByDate[date]; exists {
			point.BNRRate = &bnrRate
		}
		points = append(points, point)
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
