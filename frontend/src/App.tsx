import { useEffect, useMemo, useState } from 'react'
import { getComparison, getHistory } from './api'
import type { Comparison, History, Offer } from './types'

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
  ing: 'ING',
  raiffeisen: 'Raiffeisen',
  cec: 'CEC Bank',
  xtb: 'XTB',
}

type Theme = 'light' | 'dark'

function initialTheme(): Theme {
  try {
    const savedTheme = window.localStorage.getItem('undeschimb-theme')
    if (savedTheme === 'light' || savedTheme === 'dark') return savedTheme
  } catch {
    // A blocked localStorage should not prevent the calculator from loading.
  }
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
}

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

function App() {
  const [theme, setTheme] = useState<Theme>(initialTheme)
  const [amount, setAmount] = useState('1000')
  const [currency, setCurrency] = useState('EUR')
  const [direction, setDirection] = useState<'ron-to-fx' | 'fx-to-ron'>('ron-to-fx')
  const [comparison, setComparison] = useState<Comparison | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [historyProvider, setHistoryProvider] = useState('bcr')
  const [historyPeriod, setHistoryPeriod] = useState(30)
  const [history, setHistory] = useState<History | null>(null)

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
            setHistoryProvider((current) => response.offers.some((offer) => offer.provider === current) ? current : response.offers[0]?.provider ?? current)
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

  const providers = useMemo(() => comparison?.offers.map((offer) => offer.provider) ?? [], [comparison])

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
            aria-pressed={theme === 'dark'}
            aria-label={theme === 'dark' ? 'Activează modul luminos' : 'Activează modul întunecat'}
            title={theme === 'dark' ? 'Mod luminos' : 'Mod întunecat'}
            onClick={() => setTheme((current) => current === 'dark' ? 'light' : 'dark')}
          >
            <span aria-hidden="true">{theme === 'dark' ? '☀' : '☾'}</span>
            <span className="theme-toggle-label">{theme === 'dark' ? 'Luminos' : 'Întunecat'}</span>
          </button>
        </div>
      </nav>

      <section className="hero shell" id="sus">
        <div className="hero-copy">
          <p className="eyebrow">Cursuri reale. Decizii mai bune.</p>
          <h1>Unde îți rămân mai mulți bani după schimb?</h1>
          <p className="lede">Compară cursurile pentru conturi personale de la băncile mari și vezi instant diferența față de reperul BNR.</p>
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

      <section className="calculator-shell shell" aria-labelledby="calculator-title">
        <div className="calculator-heading">
          <div>
            <p className="eyebrow">CALCULATOR</p>
            <h2 id="calculator-title">Simulează schimbul tău</h2>
          </div>
          <p>Ratele sunt pentru schimburi standard între conturi personale.</p>
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
        {comparison && <OffersTable offers={comparison.offers} outputCurrency={comparison.to} />}
        {comparison?.missing_sources?.length ? <p className="source-note">Nu sunt disponibile momentan: {comparison.missing_sources.map((id) => PROVIDER_NAMES[id] ?? id).join(', ')}. Nu afișăm estimări pentru bănci.</p> : null}
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
            <p><span>03</span><b>Verifică mereu înainte</b><small>Promotiile, limitele și ratele negociate nu sunt incluse.</small></p>
          </div>
        </div>
      </section>

      <section className="history shell" id="istoric" aria-labelledby="history-title">
        <div className="section-heading">
          <div>
            <p className="eyebrow">ISTORIC</p>
            <h2 id="history-title">Cum a evoluat cursul?</h2>
          </div>
          <div className="history-controls">
            <select value={historyProvider} onChange={(event) => setHistoryProvider(event.target.value)}>
              {(providers.length ? providers : Object.keys(PROVIDER_NAMES)).map((provider) => <option value={provider} key={provider}>{PROVIDER_NAMES[provider]}</option>)}
            </select>
            {[7, 30, 90].map((days) => <button aria-pressed={historyPeriod === days} className={historyPeriod === days ? 'selected' : ''} type="button" key={days} onClick={() => setHistoryPeriod(days)}>{days}z</button>)}
          </div>
        </div>
        <HistoryChart history={history} providerName={PROVIDER_NAMES[historyProvider]} />
      </section>

      <footer className="footer shell">
        <div className="brand"><span className="brand-mark">↔</span><span>Unde<span>Schimb</span></span></div>
        <p>Informații orientative. Nu reprezintă recomandări financiare sau oferte de schimb.</p>
      </footer>
    </main>
  )
}

function OffersTable({ offers, outputCurrency }: { offers: Offer[]; outputCurrency: string }) {
  if (!offers.length) return <div className="message">Niciun curs nu este disponibil încă. Verificăm sursele la fiecare 15 minute.</div>
  return (
    <>
      <div className="table-wrap">
        <table>
          <thead><tr><th>Furnizor</th><th>Curs efectiv</th><th>Primești</th><th>Diferență față de BNR</th><th>Stare</th></tr></thead>
          <tbody>
            {offers.map((offer, index) => {
              const difference = Number(offer.difference_from_bnr_ron)
              return <tr key={offer.provider} className={index === 0 ? 'best' : ''}>
                <td><div className="provider"><b>{offer.provider_name}</b>{index === 0 && <span>Cea mai bună ofertă</span>}{offer.indicative && <small>Estimare indicativă</small>}</div></td>
                <td>{number(offer.effective_rate, 4, 4)} <small>RON</small></td>
                <td className="received">{money(offer.output_amount, outputCurrency)}</td>
                <td className={difference < 0 ? 'negative' : 'positive'}>{difference > 0 ? '+' : ''}{money(offer.difference_from_bnr_ron, 'RON')}<small>{number(offer.difference_percent, 2, 2)}%</small></td>
                <td><a href={offer.source_url} target="_blank" rel="noreferrer" className={offer.stale ? 'stale' : 'fresh'}>{offer.stale ? 'Expirat' : 'Actualizat'}<small>{relativeTime(offer.fetched_at)}</small></a>{offer.provider === 'xtb' && <small className="fee">taxă {number(offer.fee_percent, 1, 1)}%</small>}</td>
              </tr>
            })}
          </tbody>
        </table>
      </div>
      <div className="offers-mobile">
        {offers.map((offer, index) => <OfferCard key={offer.provider} offer={offer} index={index} outputCurrency={outputCurrency} />)}
      </div>
    </>
  )
}

function OfferCard({ offer, index, outputCurrency }: { offer: Offer; index: number; outputCurrency: string }) {
  const difference = Number(offer.difference_from_bnr_ron)
  const differenceClass = difference < 0 ? 'negative' : 'positive'

  return <article className={`offer-card ${index === 0 ? 'best' : ''}`}>
    <div className="offer-card-top">
      <div className="provider">
        <b>{offer.provider_name}</b>
        {index === 0 && <span>Cea mai bună ofertă</span>}
        {offer.indicative && <small>Estimare indicativă</small>}
      </div>
      <a href={offer.source_url} target="_blank" rel="noreferrer" className={`offer-status ${offer.stale ? 'stale' : 'fresh'}`}>
        {offer.stale ? 'Expirat' : 'Actualizat'}
        <small>{relativeTime(offer.fetched_at)}</small>
      </a>
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
    <div className="offer-card-rate">
      <span>Curs efectiv <b>{number(offer.effective_rate, 4, 4)} RON</b></span>
      {offer.provider === 'xtb' && <span className="fee">taxă {number(offer.fee_percent, 1, 1)}%</span>}
    </div>
  </article>
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
