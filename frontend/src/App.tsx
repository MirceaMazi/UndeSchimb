import { useEffect, useMemo, useState } from 'react'
import { getComparison, getHistory } from './api'
import type { Comparison, History, Offer, ProviderCategory, ProviderNotice, ProviderTab } from './types'

const CURRENCIES = [
  { code: 'EUR', label: 'Euro', symbol: '€' },
  { code: 'USD', label: 'Dolar american', symbol: '$' },
  { code: 'GBP', label: 'Liră sterlină', symbol: '£' },
  { code: 'CHF', label: 'Franc elvețian', symbol: 'CHF' },
]

const PROVIDER_NAMES: Record<string, string> = {
  banca_transilvania: 'Banca Transilvania',
  bcr: 'BCR',
  brd: 'BRD',
  brd_you: 'BRD — curs YOU',
  ing: 'ING',
  ing_preferential: 'ING — curs avantajos',
  raiffeisen: 'Raiffeisen',
  raiffeisen_smart_hour: 'Raiffeisen Smart Hour',
  cec: 'CEC Bank',
  xtb: 'XTB',
  tradeville: 'TradeVille',
  revolut: 'Revolut',
  tavex: 'Tavex',
  luxor_bucharest: 'Luxor București',
}

const CATEGORY_TABS: { id: ProviderTab; label: string; shortLabel: string }[] = [
  { id: 'all', label: 'Toate ofertele', shortLabel: 'Toate' },
  { id: 'banks', label: 'Bănci', shortLabel: 'Bănci' },
  { id: 'brokers', label: 'Brokeri & fintech', shortLabel: 'Brokeri' },
  { id: 'physical_exchanges', label: 'Case de schimb', shortLabel: 'Schimb' },
]

const HISTORY_FALLBACK = ['banca_transilvania', 'bcr', 'brd', 'brd_you', 'ing', 'ing_preferential', 'raiffeisen', 'cec', 'xtb', 'revolut', 'tavex', 'luxor_bucharest']

const SPECIAL_PROVIDER_IDS = ['ing_preferential', 'raiffeisen_smart_hour', 'brd_you'] as const
type SpecialProviderID = typeof SPECIAL_PROVIDER_IDS[number]

type Theme = 'light' | 'dark'

function initialTheme(): Theme {
  if (typeof window === 'undefined') return 'light'
  try {
    const savedTheme = window.localStorage.getItem('undeschimb-theme')
    if (savedTheme === 'light' || savedTheme === 'dark') return savedTheme
  } catch {
    // A blocked localStorage should not prevent the calculator from loading.
  }
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
}

function initialCurrency() {
  if (typeof window === 'undefined') return 'EUR'
  const currency = new URLSearchParams(window.location.search).get('currency')
  if (currency && CURRENCIES.some((item) => item.code === currency)) return currency
  return 'EUR'
}

function initialDirection(): 'ron-to-fx' | 'fx-to-ron' {
  if (typeof window === 'undefined') return 'ron-to-fx'
  return new URLSearchParams(window.location.search).get('direction') === 'fx-to-ron' ? 'fx-to-ron' : 'ron-to-fx'
}

const CURRENCY_GUIDES = [
  { code: 'EUR', label: 'Curs EUR / RON', href: '/curs-eur-ron/' },
  { code: 'USD', label: 'Curs USD / RON', href: '/curs-usd-ron/' },
  { code: 'GBP', label: 'Curs GBP / RON', href: '/curs-gbp-ron/' },
  { code: 'CHF', label: 'Curs CHF / RON', href: '/curs-chf-ron/' },
]

const FAQ_ITEMS = [
  {
    question: 'Cursul BNR poate fi folosit pentru a face schimbul valutar?',
    answer: 'Nu. Cursul BNR este un reper informativ, nu o ofertă de schimb pentru persoane fizice. Banca sau furnizorul ales stabilește cursul efectiv aplicat.',
  },
  {
    question: 'Ce cursuri sunt incluse în comparație?',
    answer: 'Clasamentul băncilor folosește cursurile standard pentru conturi personale. Cursurile preferențiale și promoțiile apar separat, cu condițiile lor, iar brokerii și casele de schimb au file dedicate.',
  },
  {
    question: 'Cum văd cursurile speciale ING, BRD și Raiffeisen?',
    answer: 'Activează comutatoarele „Oferte speciale” deasupra clasamentului. O ofertă inactivă este clasată după rezultatul estimat, dar rămâne marcată clar ca indisponibilă acum.',
  },
  {
    question: 'Cum aleg cel mai bun curs valutar când vreau să încep să investesc?',
    answer: 'Compară suma netă care ajunge în moneda contului de investiții, nu doar cursul afișat. Verifică diferența față de BNR, spreadul, comisionul de conversie și dacă oferta este disponibilă pentru suma și momentul ales.',
  },
  {
    question: 'De ce oferta XTB este marcată ca indicativă?',
    answer: 'XTB folosește cotații Standard care se pot modifica mai repede decât verificarea noastră. Calculul include comisionul de conversie publicat, însă prețul executabil din platformă poate diferi.',
  },
  {
    question: 'Cât de des sunt actualizate datele?',
    answer: 'Colectorul verifică sursele la fiecare 15 minute cât timp serviciul este activ. Verifică mereu marcajul de timp și avertizarea de date expirate înainte de a lua o decizie.',
  },
]

function number(value: string | number, minimumFractionDigits = 2, maximumFractionDigits = 4) {
  return new Intl.NumberFormat('ro-RO', { minimumFractionDigits, maximumFractionDigits }).format(Number(value))
}

function money(value: string | number, currency: string) {
  return new Intl.NumberFormat('ro-RO', {
    style: 'currency', currency, minimumFractionDigits: 2, maximumFractionDigits: 2,
  }).format(Number(value))
}

function relativeTime(value: string) {
  const seconds = Math.max(0, Math.round((Date.now() - new Date(value).getTime()) / 1000))
  if (seconds < 60) return 'chiar acum'
  if (seconds < 3600) return `acum ${Math.floor(seconds / 60)} min`
  return `acum ${Math.floor(seconds / 3600)} h`
}

function orderOffers(offers: Offer[]) {
  return [...offers].sort((left, right) => Number(right.output_amount) - Number(left.output_amount))
}

function App() {
  const [theme, setTheme] = useState<Theme>(initialTheme)
  const [amount, setAmount] = useState('1000')
  const [currency, setCurrency] = useState(initialCurrency)
  const [direction, setDirection] = useState<'ron-to-fx' | 'fx-to-ron'>(initialDirection)
  const [comparison, setComparison] = useState<Comparison | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [historyProvider, setHistoryProvider] = useState('bcr')
  const [historyPeriod, setHistoryPeriod] = useState(30)
  const [history, setHistory] = useState<History | null>(null)
  const [activeCategory, setActiveCategory] = useState<ProviderTab>('all')
  const [expandedBenefitProvider, setExpandedBenefitProvider] = useState<string | null>(null)
  const [historyExpanded, setHistoryExpanded] = useState(false)
  const [enabledSpecialOffers, setEnabledSpecialOffers] = useState<Record<SpecialProviderID, boolean>>({
    ing_preferential: false,
    raiffeisen_smart_hour: false,
    brd_you: false,
  })

  const from = direction === 'ron-to-fx' ? 'RON' : currency
  const to = direction === 'ron-to-fx' ? currency : 'RON'
  const historySide = direction === 'ron-to-fx' ? 'sell' : 'buy'
  const validAmount = Number(amount) > 0
  const resultTitle = comparison
    ? `Pentru ${number(comparison.input_amount)} ${comparison.from}, primești`
    : loading ? 'Căutăm cele mai bune cursuri…' : `Pentru ${number(amount)} ${from}, primești`

  useEffect(() => {
    document.documentElement.dataset.theme = theme
    document.querySelector('meta[name="theme-color"]')?.setAttribute('content', theme === 'dark' ? '#081d24' : '#0c2532')
    try {
      window.localStorage.setItem('undeschimb-theme', theme)
    } catch {
      // The selected theme remains active for this session if storage is unavailable.
    }
  }, [theme])

  useEffect(() => {
    if (!validAmount) {
      setComparison(null)
      setLoading(false)
      return
    }
    const controller = new AbortController()
    const timer = window.setTimeout(() => {
      setLoading(true)
      getComparison(from, to, amount)
        .then((response) => {
          if (!controller.signal.aborted) {
            setComparison(response)
            setError(null)
            const historyOffers = response.offers.filter((offer) => offer.offer_type !== 'special' && offer.provider !== 'tradeville')
            setHistoryProvider((current) => historyOffers.some((offer) => offer.provider === current) ? current : historyOffers[0]?.provider ?? current)
          }
        })
        .catch((reason: Error) => !controller.signal.aborted && setError(reason.message))
        .finally(() => !controller.signal.aborted && setLoading(false))
    }, 250)
    return () => {
      controller.abort()
      window.clearTimeout(timer)
    }
  }, [amount, from, to, validAmount])

  useEffect(() => {
    getHistory(historyProvider, currency, historySide, historyPeriod)
      .then(setHistory)
      .catch(() => setHistory(null))
  }, [currency, historyProvider, historyPeriod, historySide])

  const providers = useMemo(() => {
    const available = comparison?.offers
      .filter((offer) => offer.offer_type !== 'special' && offer.provider !== 'tradeville')
      .map((offer) => offer.provider) ?? []
    return [...new Set(available)]
  }, [comparison])
  const rankedOffers = orderOffers(comparison?.offers.filter((offer) => {
    if (activeCategory !== 'all' && offer.category !== activeCategory) return false
    if (offer.provider in enabledSpecialOffers) return enabledSpecialOffers[offer.provider as SpecialProviderID]
    return offer.offer_type !== 'preferential' && offer.offer_type !== 'special'
  }) ?? [])
  const benefitOffers = comparison?.offers.filter((offer) => activeCategory === 'all' || offer.category === activeCategory) ?? []
  const categoryNotices = comparison?.provider_notices?.filter((notice) => activeCategory === 'all' || notice.category === activeCategory) ?? []
  const informationNotices = categoryNotices.filter((notice) => notice.kind === 'quote_required' || notice.kind === 'ineligible')

  const showProviderBenefit = (provider: string) => {
    setExpandedBenefitProvider(provider)
    window.setTimeout(() => document.getElementById(`benefit-${provider}`)?.scrollIntoView({ behavior: 'smooth', block: 'center' }), 0)
  }

  return (
    <main>
      <nav className="nav shell" aria-label="Navigație principală">
        <a className="brand" href="#sus" aria-label="UndeSchimb, începutul paginii">
          <span className="brand-mark">↔</span>
          <span>Unde<span>Schimb</span></span>
        </a>
        <div className="nav-actions">
          <a className="nav-link" href="#istoric">Istoric cursuri</a>
          <button
            type="button"
            className="theme-toggle"
            aria-label="Schimbă tema"
            title="Schimbă tema"
            onClick={() => setTheme((current) => current === 'dark' ? 'light' : 'dark')}
          >
            <span aria-hidden="true">◐</span>
            <span className="theme-toggle-label">Temă</span>
          </button>
        </div>
      </nav>

      <section className="hero shell" id="sus">
        <div className="hero-copy">
          <p className="eyebrow">Cursuri reale. Decizii mai bune.</p>
          <h1>Unde îți rămân mai mulți bani după schimb?</h1>
          <p className="lede">Compară cursurile băncilor, brokerilor și caselor de schimb și vezi instant diferența față de reperul BNR.</p>
          <div className="trust-line"><span>●</span> Actualizat automat la 15 minute <i /> <span>●</span> Fără cont, fără comisioane</div>
        </div>
        <div className="rate-card" aria-label="Reper BNR">
          <div className="rate-card-top"><span>REPER OFICIAL</span><strong>BNR</strong></div>
          <p>{currency} / RON</p>
          <b>{comparison ? number(comparison.bnr_rate, 4, 4) : '—'}</b>
          <small>{comparison ? `publicat ${relativeTime(comparison.bnr_fetched_at)}` : 'se încarcă…'}</small>
          <div className="rate-card-grid"><span>Nu este curs executabil</span><span>folosit doar ca reper</span></div>
        </div>
      </section>

      <section className="calculator-shell shell" id="calculator" aria-labelledby="calculator-title">
        <div className="calculator-heading">
          <div>
            <p className="eyebrow">CALCULATOR</p>
            <h2 id="calculator-title">Simulează schimbul tău</h2>
          </div>
          <p>Alege categoria potrivită după ce introduci suma și moneda.</p>
        </div>
        <div className="calculator">
          <label className="amount-field">
            <span>Sumă de schimbat</span>
            <div>
              <input value={amount} type="number" min="0.01" step="0.01" inputMode="decimal" onChange={(event) => setAmount(event.target.value)} />
              <strong>{from}</strong>
            </div>
          </label>
          <div className="swap-control" aria-label="Direcția conversiei">
            <button type="button" aria-pressed={direction === 'ron-to-fx'} className={direction === 'ron-to-fx' ? 'active' : ''} onClick={() => setDirection('ron-to-fx')}>RON → valută</button>
            <button type="button" aria-pressed={direction === 'fx-to-ron'} className={direction === 'fx-to-ron' ? 'active' : ''} onClick={() => setDirection('fx-to-ron')}>Valută → RON</button>
          </div>
          <label className="currency-field">
            <span>Moneda</span>
            <select value={currency} onChange={(event) => setCurrency(event.target.value)}>
              {CURRENCIES.map((item) => <option key={item.code} value={item.code}>{item.code} — {item.label}</option>)}
            </select>
          </label>
        </div>
        {!validAmount && <p className="input-error">Introdu o sumă mai mare decât zero.</p>}
      </section>

      <section className="results shell" aria-live="polite">
        <div className="section-heading">
          <div>
            <p className="eyebrow">COMPARAȚIE</p>
            <h2>{resultTitle}</h2>
          </div>
          {comparison && <p className="benchmark">Reper BNR: <strong>{money(comparison.bnr_output, comparison.to)}</strong></p>}
        </div>

        {error && <div className="message error"><b>Nu am putut calcula comparația.</b> {error}</div>}
        {loading && !comparison && <div className="message">Se încarcă sursele disponibile…</div>}
        {comparison && <>
          <ProviderTabs activeCategory={activeCategory} onChange={setActiveCategory} />
          {(activeCategory === 'all' || activeCategory === 'banks') && <SpecialOfferToggles
            currency={currency}
            enabled={enabledSpecialOffers}
            onChange={(provider, checked) => setEnabledSpecialOffers((current) => ({ ...current, [provider]: checked }))}
          />}
          <div className="ranking-heading">
            <div>
              <h3>{activeCategory === 'all' ? 'Clasament general' : activeCategory === 'banks' ? 'Clasament cursuri standard' : activeCategory === 'brokers' ? 'Cotații comparabile' : 'Cursuri pentru numerar'}</h3>
              <p>{activeCategory === 'all'
                ? 'Toți furnizorii sunt reuniți aici; ofertele speciale apar numai dacă le activezi și rămân marcate clar.'
                : activeCategory === 'banks'
                ? 'Activează doar avantajele pentru care ești eligibil; ofertele inactive rămân vizibile ca previzualizare.'
                : activeCategory === 'brokers'
                  ? 'Sunt ordonate doar ofertele pentru care există o cotație publică.'
                  : 'Sunt clasate numai cursurile pentru care suma ta îndeplinește pragurile publicate.'}</p>
            </div>
          </div>
          <OffersTable
            offers={rankedOffers}
            informationNotices={informationNotices}
            benefitProviders={categoryNotices.map((notice) => notice.provider)}
            outputCurrency={comparison.to}
            showCategories={activeCategory === 'all'}
            onShowConditions={showProviderBenefit}
          />
          {categoryNotices.length > 0 && <ProviderBenefits
            notices={categoryNotices}
            offers={benefitOffers}
            outputCurrency={comparison.to}
            expandedProvider={expandedBenefitProvider}
            onExpandedProviderChange={setExpandedBenefitProvider}
          />}
        </>}
        {comparison?.missing_sources?.length ? <p className="source-note">Nu sunt disponibile momentan: {comparison.missing_sources.map((id) => PROVIDER_NAMES[id] ?? id).join(', ')}. Păstrăm doar ultimul curs valid, marcat ca expirat; nu inventăm estimări.</p> : null}
      </section>

      <section className="seo-intro shell" aria-labelledby="seo-title">
        <p className="eyebrow">GHID DE SCHIMB VALUTAR</p>
        <h2 id="seo-title">Curs valutar: cum alegi oferta potrivită?</h2>
        <div className="seo-copy">
          <p>La un schimb valutar contează suma pe care o primești, nu doar cifra afișată ca „curs”. Pentru RON spre valută, un curs de vânzare mai mic înseamnă mai multă valută primită. Pentru valută spre RON, un curs de cumpărare mai mare înseamnă mai mulți lei primiți.</p>
          <p>UndeSchimb separă cursurile standard de avantajele condiționate și grupează băncile, brokerii și casele de schimb. Astfel compari oferte similare și vezi exact ce condiții trebuie îndeplinite.</p>
        </div>
        <div className="investment-guide">
          <div>
            <p className="eyebrow">SCHIMB VALUTAR PENTRU INVESTIȚII</p>
            <h3>Găsește un curs mai bun înainte să începi să investești</h3>
          </div>
          <div>
            <p>Dacă alimentezi în EUR sau USD un cont de investiții din venituri în RON, conversia valutară este unul dintre primele costuri pe care le suporți. Comparatorul te ajută să vezi câtă valută ajunge efectiv la tine printr-o bancă, un broker sau un serviciu fintech.</p>
            <p>Pentru investiții recurente, diferențele mici de curs și comisioanele repetate se pot aduna în timp. Compară rezultatul net, diferența față de BNR, programul ofertelor speciale și eventualele taxe înainte de fiecare transfer. Cotația executabilă din platforma aleasă rămâne cea care se aplică tranzacției.</p>
            <small>Informațiile sunt orientative și nu reprezintă recomandări de investiții.</small>
          </div>
        </div>
        <div className="currency-guides" aria-label="Ghiduri pentru monede">
          {CURRENCY_GUIDES.map((guide) => <a key={guide.code} href={guide.href}><span>{guide.code}</span>{guide.label}<b aria-hidden="true">→</b></a>)}
        </div>
      </section>

      <section className="methodology shell" id="metodologie" aria-labelledby="methodology-title">
        <div>
          <p className="eyebrow">METODOLOGIE</p>
          <h2 id="methodology-title">Cum calculăm comparația</h2>
        </div>
        <ol>
          <li><span>01</span><p><b>Normalizăm cursurile</b><small>Toate valorile sunt exprimate ca RON pentru o unitate de monedă străină.</small></p></li>
          <li><span>02</span><p><b>Aplicăm direcția corectă</b><small>Împărțim la cursul de vânzare când cumperi valută și înmulțim cu cursul de cumpărare când vinzi valută.</small></p></li>
          <li><span>03</span><p><b>Arătăm diferența reală</b><small>Comparăm rezultatul fiecărei oferte cu echivalentul BNR și afișăm diferența în lei.</small></p></li>
        </ol>
        <a className="text-link" href="/metodologie/">Citește metodologia completă <span aria-hidden="true">→</span></a>
      </section>

      <section className="how-it-works">
        <div className="shell how-grid">
          <div>
            <p className="eyebrow">CUM CITIM REZULTATELE</p>
            <h2>Un curs mai mic nu este mereu mai bun.</h2>
            <p>Pentru a cumpăra valută, contează cursul de vânzare al furnizorului. Pentru a vinde valută, contează cursul de cumpărare. UndeSchimb aplică automat direcția corectă.</p>
          </div>
          <div className="rules">
            <p><span>01</span><b>BNR este un reper</b><small>Cursul BNR nu este o ofertă de schimb pentru persoane fizice.</small></p>
            <p><span>02</span><b>XTB este indicativ</b><small>Folosește cotații Standard publice și taxa de conversie activă.</small></p>
            <p><span>03</span><b>Beneficiile au condiții</b><small>Promoțiile, pachetele și intervalele orare sunt explicate separat de cursul standard.</small></p>
          </div>
        </div>
      </section>

      <section className={`history shell ${historyExpanded ? 'expanded' : ''}`} id="istoric" aria-labelledby="history-title">
        <div className="section-heading">
          <div>
            <p className="eyebrow">ISTORIC</p>
            <h2 id="history-title">Cum a evoluat cursul?</h2>
          </div>
          <HistoryControls className="history-controls-desktop" providers={providers} provider={historyProvider} period={historyPeriod} onProviderChange={setHistoryProvider} onPeriodChange={setHistoryPeriod} />
        </div>
        <button type="button" className="history-mobile-toggle" aria-expanded={historyExpanded} aria-controls="history-mobile-panel" onClick={() => setHistoryExpanded((current) => !current)}>
          <span><b>Vezi evoluția cursului</b><small>{PROVIDER_NAMES[historyProvider]} · ultimele {historyPeriod} zile</small></span>
          <span className="disclosure-icon" aria-hidden="true">⌄</span>
        </button>
        <div className="history-mobile-panel" id="history-mobile-panel">
          <HistoryControls className="history-controls-mobile" providers={providers} provider={historyProvider} period={historyPeriod} onProviderChange={setHistoryProvider} onPeriodChange={setHistoryPeriod} />
          <HistoryChart history={history} providerName={PROVIDER_NAMES[historyProvider]} />
        </div>
      </section>

      <section className="faq shell" id="intrebari" aria-labelledby="faq-title">
        <p className="eyebrow">ÎNTREBĂRI FRECVENTE</p>
        <h2 id="faq-title">Lucruri utile înainte de schimb</h2>
        <div className="faq-list">
          {FAQ_ITEMS.map((item, index) => <details key={item.question} open={index === 0}>
            <summary>{item.question}<span aria-hidden="true">+</span></summary>
            <p>{item.answer}</p>
          </details>)}
        </div>
      </section>

      <footer className="footer shell">
        <div className="brand"><span className="brand-mark">↔</span><span>Unde<span>Schimb</span></span></div>
        <p>Informații orientative. Nu reprezintă recomandări financiare sau oferte de schimb.</p>
      </footer>
    </main>
  )
}

function ProviderTabs({
  activeCategory,
  onChange,
}: {
  activeCategory: ProviderTab
  onChange: (category: ProviderTab) => void
}) {
  return <div className="provider-tabs" role="tablist" aria-label="Tipul furnizorului">
    {CATEGORY_TABS.map((tab) => <button
        type="button"
        role="tab"
        id={`tab-${tab.id}`}
        aria-selected={activeCategory === tab.id}
        aria-controls="provider-results"
        className={activeCategory === tab.id ? 'active' : ''}
        key={tab.id}
        onClick={() => onChange(tab.id)}
      >
        <span className="tab-label-full">{tab.label}</span>
        <span className="tab-label-short">{tab.shortLabel}</span>
      </button>
    )}
  </div>
}

function SpecialOfferToggles({
  currency,
  enabled,
  onChange,
}: {
  currency: string
  enabled: Record<SpecialProviderID, boolean>
  onChange: (provider: SpecialProviderID, checked: boolean) => void
}) {
  const options: { provider: SpecialProviderID; label: string; detail: string; supported: boolean }[] = [
    { provider: 'ing_preferential', label: 'ING curs avantajos', detail: 'În funcție de pachet și limita lunară', supported: true },
    { provider: 'raiffeisen_smart_hour', label: 'Raiffeisen Smart Hour', detail: currency === 'EUR' ? 'EUR/RON · 10:00–11:00' : 'Disponibil doar pentru EUR/RON', supported: currency === 'EUR' },
    { provider: 'brd_you', label: 'BRD YOU', detail: currency === 'EUR' ? 'EUR/RON · disponibil 24/7' : 'Disponibil doar pentru EUR/RON', supported: currency === 'EUR' },
  ]
  const enabledCount = options.filter((option) => option.supported && enabled[option.provider]).length
  const toggles = options.map((option) => <label className={`special-toggle ${!option.supported ? 'disabled' : ''}`} key={option.provider}>
    <input
      type="checkbox"
      checked={option.supported && enabled[option.provider]}
      disabled={!option.supported}
      onChange={(event) => onChange(option.provider, event.target.checked)}
    />
    <span className="switch-track" aria-hidden="true"><span /></span>
    <span className="special-toggle-copy"><b>{option.label}</b><small>{option.detail}</small></span>
  </label>)

  return <>
    <section className="special-offer-controls special-offers-desktop" aria-labelledby="special-offers-title">
      <div className="special-offer-heading">
        <div><p className="eyebrow">OFERTE SPECIALE</p><h3 id="special-offers-title">Include avantajele tale</h3></div>
        <p>Activează numai ofertele ale căror condiții le îndeplinești.</p>
      </div>
      <div className="special-toggle-grid">{toggles}</div>
    </section>
    <details className="special-offer-controls special-offers-mobile">
      <summary>
        <span><b>Ofertele mele speciale</b><small>{enabledCount ? `${enabledCount} ${enabledCount === 1 ? 'ofertă activată' : 'oferte activate'}` : 'ING, Raiffeisen și BRD'}</small></span>
        {enabledCount > 0 && <strong>{enabledCount}</strong>}
        <span className="disclosure-icon" aria-hidden="true">⌄</span>
      </summary>
      <p>Activează numai ofertele ale căror condiții le îndeplinești.</p>
      <div className="special-toggle-grid">{toggles}</div>
    </details>
  </>
}

function ProviderBenefits({
  notices,
  offers,
  outputCurrency,
  expandedProvider,
  onExpandedProviderChange,
}: {
  notices: ProviderNotice[]
  offers: Offer[]
  outputCurrency: string
  expandedProvider: string | null
  onExpandedProviderChange: (provider: string | null) => void
}) {
  return <section className="provider-benefits" aria-labelledby="benefits-title">
    <div className="benefits-heading">
      <div>
        <p className="eyebrow">AVANTAJE ȘI CONDIȚII</p>
        <h3 id="benefits-title">Merită să știi că există</h3>
      </div>
      <p>Aici explicăm pragurile, programul și condițiile care pot modifica oferta din clasament.</p>
    </div>
    <div className="benefit-grid">
      {notices.map((notice) => {
        const offer = offers.find((item) => item.provider === notice.provider)
        const status = notice.kind === 'scheduled'
          ? notice.active_now
            ? notice.request_eligible ? 'Activ acum' : 'Suma depășește limita'
            : 'Interval limitat'
          : notice.kind === 'quote_required'
            ? 'Verifică în platformă'
            : notice.kind === 'ineligible'
              ? 'Prag neîndeplinit'
              : notice.category === 'physical_exchanges'
                ? 'Curs aplicabil'
                : notice.provider === 'revolut'
                  ? 'Verifică planul'
                  : notice.provider === 'cec_digital'
                    ? 'Inclus în clasament'
                    : notice.provider === 'brd_you'
                      ? notice.request_eligible ? 'Disponibil 24/7' : 'Doar EUR/RON'
                      : notice.provider === 'tradeville'
                        ? notice.active_now ? 'Interval activ' : 'Interval 09:00–16:00'
                        : notice.request_eligible ? 'Verifică în aplicație' : 'Condiții neîndeplinite'
        const statusClass = notice.active_now && notice.request_eligible ? 'active' : notice.request_eligible ? 'conditional' : 'unavailable'
        const difference = offer ? Number(offer.difference_from_bnr_ron) : 0
        const expanded = expandedProvider === notice.provider

        return <article className={`benefit-card ${expanded ? 'expanded' : ''}`} id={`benefit-${notice.provider}`} key={notice.provider}>
          <button type="button" className="benefit-mobile-summary" aria-expanded={expanded} onClick={() => onExpandedProviderChange(expanded ? null : notice.provider)}>
            <span><small>{notice.provider_name}</small><b>{notice.title}</b></span>
            <span className={`benefit-status ${statusClass}`}>{status}</span>
            <span className="disclosure-icon" aria-hidden="true">⌄</span>
          </button>
          <div className="benefit-card-top">
            <div><small>{notice.provider_name}</small><h4>{notice.title}</h4></div>
            <span className={`benefit-status ${statusClass}`}>{status}</span>
          </div>
          <div className="benefit-card-body">
            <p>{notice.description}</p>
            {offer && <div className="benefit-result">
              <span>{offer.active_now && offer.request_eligible ? 'Estimare pentru suma ta' : 'Previzualizare pentru suma ta'}</span>
              <strong>{money(offer.output_amount, outputCurrency)}</strong>
              <small className={difference < 0 ? 'negative' : 'positive'}>
                {difference > 0 ? '+' : ''}{money(offer.difference_from_bnr_ron, 'RON')} față de BNR
              </small>
            </div>}
            <ul>{(notice.conditions ?? []).map((condition) => <li key={condition}>{condition}</li>)}</ul>
            <a href={notice.source_url} target="_blank" rel="noreferrer">Vezi condițiile oficiale <span aria-hidden="true">↗</span></a>
          </div>
        </article>
      })}
    </div>
  </section>
}

function OffersTable({
  offers,
  informationNotices,
  benefitProviders,
  outputCurrency,
  showCategories,
  onShowConditions,
}: {
  offers: Offer[]
  informationNotices: ProviderNotice[]
  benefitProviders: string[]
  outputCurrency: string
  showCategories: boolean
  onShowConditions: (provider: string) => void
}) {
  const [showAllMobile, setShowAllMobile] = useState(false)
  const offersKey = offers.map((offer) => offer.provider).join('|')

  useEffect(() => setShowAllMobile(false), [offersKey])

  if (!offers.length && !informationNotices.length) return <div className="message">Niciun curs nu este disponibil încă. Verificăm sursele la fiecare 15 minute.</div>
  const bestProvider = offers
    .reduce<Offer | null>((best, offer) => (
      !best || Number(offer.output_amount) > Number(best.output_amount) ? offer : best
    ), null)?.provider
  const mobileLimit = 4
  const totalMobileResults = offers.length + informationNotices.length
  const visibleMobileOffers = showAllMobile ? offers : offers.slice(0, mobileLimit)
  const remainingMobileSlots = Math.max(0, mobileLimit - visibleMobileOffers.length)
  const visibleMobileNotices = showAllMobile ? informationNotices : informationNotices.slice(0, remainingMobileSlots)
  return (
    <div id="provider-results" role="tabpanel">
      <div className="table-wrap">
        <table>
          <thead><tr><th>Furnizor</th><th>Curs efectiv</th><th>Primești</th><th>Diferență față de BNR</th><th>Stare</th></tr></thead>
          <tbody>
            {offers.map((offer) => {
              const difference = Number(offer.difference_from_bnr_ron)
              const isAvailable = offer.active_now && offer.request_eligible
              const isBest = offers.length > 1 && offer.provider === bestProvider
              return <tr key={offer.provider} className={`${isBest ? 'best' : ''} ${!isAvailable ? 'offer-unavailable' : ''}`}>
                <td><div className="provider"><b>{offer.provider_name}</b>{showCategories && <ProviderCategoryLabel category={offer.category} />}{isBest && <span className="best-label">{isAvailable ? 'Cea mai bună ofertă disponibilă' : 'Cel mai bun curs · indisponibil acum'}</span>}<OfferTypeLabel offer={offer} /><LocationLabel offer={offer} /></div></td>
                <td>{number(offer.effective_rate, 4, 4)} <small>RON</small></td>
                <td className="received">{money(offer.output_amount, outputCurrency)}</td>
                <td className={difference < 0 ? 'negative' : 'positive'}>{difference > 0 ? '+' : ''}{money(offer.difference_from_bnr_ron, 'RON')}<small>{number(offer.difference_percent, 2, 2)}%</small></td>
                <td><a href={offer.source_url} target="_blank" rel="noreferrer" className={!isAvailable ? 'unavailable' : offer.stale ? 'stale' : 'fresh'}>{!offer.request_eligible ? 'Suma neeligibilă' : !offer.active_now ? 'Indisponibil acum' : offer.stale ? 'Expirat' : 'Actualizat'}<small>{isAvailable ? relativeTime(offer.fetched_at) : 'vezi condițiile'}</small></a>{offer.provider === 'xtb' && <small className="fee">taxă {number(offer.fee_percent, 1, 1)}%</small>}</td>
              </tr>
            })}
            {informationNotices.map((notice) => <InformationRow key={notice.provider} notice={notice} showCategory={showCategories} />)}
          </tbody>
        </table>
      </div>
      <div className="offers-mobile">
        {visibleMobileOffers.map((offer, index) => <OfferCard
          key={offer.provider}
          offer={offer}
          rank={index + 1}
          isBest={offers.length > 1 && offer.provider === bestProvider}
          hasBenefit={benefitProviders.includes(offer.provider)}
          outputCurrency={outputCurrency}
          showCategory={showCategories}
          onShowConditions={onShowConditions}
        />)}
        {visibleMobileNotices.map((notice) => <InformationCard key={notice.provider} notice={notice} showCategory={showCategories} />)}
        {totalMobileResults > mobileLimit && <button type="button" className="show-all-offers" aria-expanded={showAllMobile} onClick={() => setShowAllMobile((current) => !current)}>
          {showAllMobile ? 'Arată mai puține' : `Vezi toate ofertele (${totalMobileResults})`}
          <span className="disclosure-icon" aria-hidden="true">⌄</span>
        </button>}
      </div>
    </div>
  )
}

function ProviderCategoryLabel({ category }: { category: ProviderCategory }) {
  const labels: Record<ProviderCategory, string> = {
    banks: 'Bancă',
    brokers: 'Broker / fintech',
    physical_exchanges: 'Casă de schimb',
  }
  return <small className={`category-badge category-${category}`}>{labels[category]}</small>
}

function LocationLabel({ offer }: { offer: Offer }) {
  if (!offer.location_label) return null
  return <small className={`location-label location-${offer.location_policy}`}>
    <b>{offer.location_label}</b>
    <span>{offer.location_note}</span>
  </small>
}

function InformationRow({ notice, showCategory }: { notice: ProviderNotice; showCategory: boolean }) {
  const ineligible = notice.kind === 'ineligible'
  return <tr className={`information-row ${ineligible ? 'ineligible-row' : ''}`}>
    <td><div className="provider"><b>{notice.provider_name}</b>{showCategory && <ProviderCategoryLabel category={notice.category} />}<small>{ineligible ? 'Prag neîndeplinit' : 'Cotație în aplicație'}</small></div></td>
    <td className="unavailable-value">—</td>
    <td className="unavailable-value">{ineligible ? 'Suma este prea mică' : 'Verifică în platformă'}</td>
    <td className="unavailable-value">—</td>
    <td><a href={notice.source_url} target="_blank" rel="noreferrer" className="verify-link">Sursă oficială<small>{ineligible ? 'vezi pragul' : 'curs dinamic'}</small></a></td>
  </tr>
}

function InformationCard({ notice, showCategory }: { notice: ProviderNotice; showCategory: boolean }) {
  const ineligible = notice.kind === 'ineligible'
  return <article className={`offer-card information-card ${ineligible ? 'ineligible-card' : ''}`}>
    <div className="offer-card-top">
      <div className="provider"><b>{notice.provider_name}</b>{showCategory && <ProviderCategoryLabel category={notice.category} />}<small>{ineligible ? 'Prag neîndeplinit' : 'Cotație în aplicație'}</small></div>
      <span className="offer-availability unavailable">{ineligible ? 'Neeligibil' : 'În aplicație'}</span>
    </div>
    <strong>{ineligible ? 'Suma nu îndeplinește pragul' : 'Verifică suma exactă în platformă'}</strong>
    <details className="offer-details">
      <summary><span>Detalii și sursă</span><span className="disclosure-icon" aria-hidden="true">⌄</span></summary>
      <div className="offer-details-content">
        <p>{notice.description}</p>
        <a href={notice.source_url} target="_blank" rel="noreferrer" className="mobile-source-link">Sursă oficială <span aria-hidden="true">↗</span></a>
      </div>
    </details>
  </article>
}

function OfferTypeLabel({ offer }: { offer: Offer }) {
  if (offer.offer_type === 'cash') return <small>Schimb cu numerar</small>
  if (offer.offer_type === 'preferential') return <small>Curs preferențial activat</small>
  if (offer.offer_type === 'special') return <small>Ofertă specială activată</small>
  if (offer.indicative) return <small>Estimare indicativă</small>
  return null
}

function OfferCard({
  offer,
  rank,
  isBest,
  hasBenefit,
  outputCurrency,
  showCategory,
  onShowConditions,
}: {
  offer: Offer
  rank: number
  isBest: boolean
  hasBenefit: boolean
  outputCurrency: string
  showCategory: boolean
  onShowConditions: (provider: string) => void
}) {
  const difference = Number(offer.difference_from_bnr_ron)
  const differenceClass = difference < 0 ? 'negative' : 'positive'
  const isAvailable = offer.active_now && offer.request_eligible
  const status = !offer.request_eligible ? 'Suma neeligibilă' : !offer.active_now ? 'Indisponibil acum' : offer.stale ? 'Expirat' : 'Actualizat'

  return <article className={`offer-card ${isBest ? 'best' : ''} ${!isAvailable ? 'offer-unavailable' : ''}`}>
    <div className="offer-card-top">
      <span className="offer-rank" aria-label={`Locul ${rank}`}>#{rank}</span>
      <div className="provider">
        <b>{offer.provider_name}</b>
        {isBest && <span className="best-label">{isAvailable ? 'Cea mai bună ofertă disponibilă' : 'Cel mai bun curs · indisponibil acum'}</span>}
      </div>
      <span className={`offer-availability ${!isAvailable ? 'unavailable' : offer.stale ? 'stale' : 'fresh'}`}>{status}<small>{isAvailable ? relativeTime(offer.fetched_at) : 'verifică programul'}</small></span>
    </div>
    <div className="offer-card-results">
      <div>
        <span>Primești</span>
        <strong>{money(offer.output_amount, outputCurrency)}</strong>
      </div>
      <div className={differenceClass}>
        <span>Față de BNR</span>
        <strong>{difference > 0 ? '+' : ''}{money(offer.difference_from_bnr_ron, 'RON')}</strong>
        <small>{number(offer.difference_percent, 2, 2)}%</small>
      </div>
    </div>
    <details className="offer-details">
      <summary><span>Curs, sursă și condiții</span><span className="disclosure-icon" aria-hidden="true">⌄</span></summary>
      <div className="offer-details-content">
        <div className="offer-detail-chips">
          {showCategory && <ProviderCategoryLabel category={offer.category} />}
          <OfferTypeLabel offer={offer} />
          <span className={`detail-status ${!isAvailable ? 'unavailable' : offer.stale ? 'stale' : 'fresh'}`}>{status}</span>
        </div>
        <div className="offer-card-rate">
          <span>Curs efectiv <b>{number(offer.effective_rate, 4, 4)} RON</b></span>
          {offer.provider === 'xtb' && <span className="fee">taxă {number(offer.fee_percent, 1, 1)}%</span>}
        </div>
        <LocationLabel offer={offer} />
        {(offer.conditions?.length ?? 0) > 0 && <ul className="offer-conditions">{offer.conditions.map((condition) => <li key={condition}>{condition}</li>)}</ul>}
        <div className="offer-detail-actions">
          {hasBenefit && <button type="button" onClick={() => onShowConditions(offer.provider)}>Vezi condițiile</button>}
          <a href={offer.source_url} target="_blank" rel="noreferrer" className="mobile-source-link">Sursă oficială <span aria-hidden="true">↗</span></a>
        </div>
      </div>
    </details>
  </article>
}

function HistoryControls({
  className,
  providers,
  provider,
  period,
  onProviderChange,
  onPeriodChange,
}: {
  className: string
  providers: string[]
  provider: string
  period: number
  onProviderChange: (provider: string) => void
  onPeriodChange: (period: number) => void
}) {
  return <div className={`history-controls ${className}`}>
    <select aria-label="Furnizor pentru istoric" value={provider} onChange={(event) => onProviderChange(event.target.value)}>
      {(providers.length ? providers : HISTORY_FALLBACK).map((providerID) => <option value={providerID} key={providerID}>{PROVIDER_NAMES[providerID]}</option>)}
    </select>
    {[7, 30, 90].map((days) => <button aria-pressed={period === days} className={period === days ? 'selected' : ''} type="button" key={days} onClick={() => onPeriodChange(days)}>{days}z</button>)}
  </div>
}

function HistoryChart({ history, providerName }: { history: History | null; providerName: string }) {
  if (!history?.points.length) return <div className="chart-empty">Istoricul apare după ce sunt colectate suficiente date.</div>
  if (history.points.length === 1) {
    const item = history.points[0]
    return <div className="chart-card chart-single">
      <div className="legend"><span className="provider-dot" />{providerName}<span className="bnr-dot" />BNR <small>{history.side === 'sell' ? 'curs de vânzare' : 'curs de cumpărare'}</small></div>
      <div className="single-quote">
        <div><span>{providerName}</span><strong>{number(item.provider_rate, 4, 4)} RON</strong></div>
        <div><span>BNR</span><strong>{number(item.bnr_rate, 4, 4)} RON</strong></div>
      </div>
	  <p>Există o singură cotație salvată pentru această selecție. Graficul se va completa pe măsură ce se acumulează date în zilele următoare.</p>
    </div>
  }
  const values = history.points.flatMap((point) => [Number(point.provider_rate), Number(point.bnr_rate)])
  const min = Math.min(...values)
  const max = Math.max(...values)
  const span = max - min || 0.01
  const point = (value: number, index: number) => {
    const x = history.points.length === 1 ? 320 : 18 + (index / (history.points.length - 1)) * 604
    const y = 18 + ((max - value) / span) * 184
    return `${x.toFixed(1)},${y.toFixed(1)}`
  }
  const providerPath = history.points.map((item, index) => point(Number(item.provider_rate), index)).join(' ')
  const bnrPath = history.points.map((item, index) => point(Number(item.bnr_rate), index)).join(' ')
  return <div className="chart-card">
    <div className="legend"><span className="provider-dot" />{providerName}<span className="bnr-dot" />BNR <small>{history.side === 'sell' ? 'curs de vânzare' : 'curs de cumpărare'}</small></div>
    <svg viewBox="0 0 640 220" role="img" aria-label={`Evoluția cursului ${providerName} și BNR`}>
      {[18, 64, 110, 156, 202].map((y) => <line key={y} x1="18" x2="622" y1={y} y2={y} />)}
      <polyline points={bnrPath} className="bnr-line" />
      <polyline points={providerPath} className="provider-line" />
    </svg>
    <div className="chart-dates"><span>{new Date(history.points[0].date).toLocaleDateString('ro-RO', { day: '2-digit', month: 'short' })}</span><span>{new Date(history.points.at(-1)!.date).toLocaleDateString('ro-RO', { day: '2-digit', month: 'short' })}</span></div>
  </div>
}

export default App
