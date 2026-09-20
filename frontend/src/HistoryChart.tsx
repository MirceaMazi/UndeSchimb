import type { History } from './types'

const DAY = 86_400_000
const PLOT_WIDTH = 1000
const PLOT_HEIGHT = 240
const rateFormat = new Intl.NumberFormat('ro-RO', { minimumFractionDigits: 4, maximumFractionDigits: 4 })
const tickDateFormat = new Intl.DateTimeFormat('ro-RO', { day: '2-digit', month: '2-digit', timeZone: 'UTC' })
const fullDateFormat = new Intl.DateTimeFormat('ro-RO', { day: '2-digit', month: '2-digit', year: 'numeric', timeZone: 'UTC' })

// Use whole 0.0001 RON units so adjacent ticks remain distinct at the displayed precision.
function rateScale(values: number[]) {
  const minimum = Math.floor(Math.min(...values) * 10_000)
  const maximum = Math.ceil(Math.max(...values) * 10_000)
  const padding = Math.max((maximum - minimum) * 0.1, 2)
  const roughStep = (maximum - minimum + 2 * padding) / 4
  const magnitude = 10 ** Math.floor(Math.log10(roughStep))
  const multiple = [1, 2, 2.5, 5, 10].find((value) => value * magnitude >= roughStep) ?? 10
  const step = Math.max(1, Math.ceil(multiple * magnitude))
  const lower = Math.max(0, Math.floor((minimum - padding) / step) * step)
  const upper = Math.ceil((maximum + padding) / step) * step
  return {
    ticks: Array.from({ length: Math.round((upper - lower) / step) + 1 }, (_, index) => (upper - index * step) / 10_000),
    position: (value: number) => (upper - value * 10_000) / (upper - lower),
  }
}

export default function HistoryChart({ history, providerName, period }: { history: History | null; providerName: string; period: number }) {
  const points = (history?.points ?? []).map((point) => ({
    // The API returns calendar dates, not timestamps. UTC preserves those dates in every browser timezone.
    date: Date.parse(`${point.date}T00:00:00Z`),
    providerRate: Number(point.provider_rate),
    bnrRate: point.bnr_rate != null && Number.isFinite(Number(point.bnr_rate)) && Number(point.bnr_rate) > 0 ? Number(point.bnr_rate) : null,
  })).filter((point) => Number.isFinite(point.date)
    && Number.isFinite(point.providerRate) && point.providerRate > 0)
    .sort((left, right) => left.date - right.date)

  if (!history || !points.length) return <div className="chart-empty" role="status">
    <p>Nu există cotații colectate pentru {providerName} în ultimele {period} zile.</p>
    {period < 90 && <p>Încearcă un interval mai lung pentru a vedea observațiile mai vechi.</p>}
  </div>

  const sideLabel = history.side === 'sell' ? 'curs de vânzare' : 'curs de cumpărare'
  const unit = `RON pentru 1 ${history.currency}`
  const missingBNR = points.some((point) => point.bnrRate === null)
  const bnrWarning = missingBNR && <p className="chart-warning" role="status">Lipsesc cotații BNR pentru unele zile. Afișăm cursul furnizorului; reperul BNR apare doar unde există date colectate.</p>
  const legend = <div className="legend">
    <span className="legend-key"><span className="provider-dot" />{providerName}</span>
    <span className="legend-key"><span className="bnr-dot" />BNR</span>
    <small>{sideLabel}</small>
  </div>

  if (points.length === 1) {
    const item = points[0]
    return <div className="chart-card chart-single">
      {legend}
      <p className="chart-unit">{unit} · {fullDateFormat.format(item.date)}</p>
      <div className="single-quote">
        <div><span>{providerName}</span><strong>{rateFormat.format(item.providerRate)} RON</strong></div>
        <div><span>BNR</span><strong>{item.bnrRate === null ? 'Date lipsă' : `${rateFormat.format(item.bnrRate)} RON`}</strong></div>
      </div>
      <p>Există o singură cotație salvată pentru această selecție. Graficul se va completa pe măsură ce se acumulează date în zilele următoare.</p>
      {bnrWarning}
    </div>
  }

  const firstDate = points[0].date
  const lastDate = points[points.length - 1].date
  const dateSpan = lastDate - firstDate
  const days = Math.round(dateSpan / DAY)
  const xPosition = (date: number) => dateSpan ? (date - firstDate) / dateSpan : 0.5
  const yScale = rateScale(points.flatMap((point) => point.bnrRate === null ? [point.providerRate] : [point.providerRate, point.bnrRate]))
  const coordinates = (date: number, rate: number) => `${(xPosition(date) * PLOT_WIDTH).toFixed(2)},${(yScale.position(rate) * PLOT_HEIGHT).toFixed(2)}`
  const providerPath = points.map((point) => coordinates(point.date, point.providerRate)).join(' ')
  const bnrSegments: string[][] = []
  let bnrSegment: string[] = []
  for (const point of points) {
    if (point.bnrRate === null) {
      bnrSegment = []
    } else {
      if (!bnrSegment.length) bnrSegments.push(bnrSegment)
      bnrSegment.push(coordinates(point.date, point.bnrRate))
    }
  }
  const intervals = Math.min(4, days)
  const dateTicks = intervals ? Array.from({ length: intervals + 1 }, (_, index) => (
    firstDate + Math.round(days * index / intervals) * DAY
  )) : [firstDate]
  const middleDate = firstDate + Math.round(days / 2) * DAY

  return <div className="chart-card">
    {legend}
    <p className="chart-unit">{unit}</p>
    <div className="chart-axes">
      <div className="chart-y-axis" aria-label={`Axa cursului, ${unit}`}>
        {yScale.ticks.map((value) => <span key={value} style={{ top: `${yScale.position(value) * 100}%` }}>{rateFormat.format(value)}</span>)}
      </div>
      <div className="chart-plot">
        <svg className="history-chart" viewBox={`0 0 ${PLOT_WIDTH} ${PLOT_HEIGHT}`} preserveAspectRatio="none" role="img"
          aria-label={`${providerName} și BNR: ${sideLabel}, ${unit}, de la ${fullDateFormat.format(firstDate)} la ${fullDateFormat.format(lastDate)}`}>
          <desc>Axa orizontală arată data colectării, iar axa verticală arată cursul în lei pentru o unitate de valută. Fiecare punct reprezintă ultimul curs salvat în ziua respectivă.</desc>
          {yScale.ticks.map((value) => <line key={value} className="chart-grid-line" x1="0" x2={PLOT_WIDTH}
            y1={yScale.position(value) * PLOT_HEIGHT} y2={yScale.position(value) * PLOT_HEIGHT} />)}
          {dateTicks.map((date, index) => <line key={date} className={`chart-grid-line chart-grid-vertical${index > 0 && index < dateTicks.length - 1 && (days < 4 || date !== middleDate) ? ' chart-tick-secondary' : ''}`}
            x1={xPosition(date) * PLOT_WIDTH} x2={xPosition(date) * PLOT_WIDTH} y1="0" y2={PLOT_HEIGHT} />)}
          {bnrSegments.map((segment, index) => segment.length > 1
            ? <polyline key={index} points={segment.join(' ')} className="bnr-line" />
            : <line key={index} x1={segment[0].split(',')[0]} x2={segment[0].split(',')[0]}
              y1={segment[0].split(',')[1]} y2={segment[0].split(',')[1]} className="bnr-point" />)}
          <polyline points={providerPath} className="provider-line" />
        </svg>
      </div>
      <div className="chart-x-axis" aria-label="Axa datelor de colectare">
        {dateTicks.map((date, index) => <span key={date}
          className={`${index === 0 ? 'chart-tick-first' : index === dateTicks.length - 1 ? 'chart-tick-last' : ''}${index > 0 && index < dateTicks.length - 1 && (days < 4 || date !== middleDate) ? ' chart-tick-secondary' : ''}`}
          style={{ left: `${xPosition(date) * 100}%` }}>
          <time dateTime={new Date(date).toISOString().slice(0, 10)}>{tickDateFormat.format(date)}</time>
        </span>)}
      </div>
    </div>
    <p className="chart-caption">Data colectării · {fullDateFormat.format(firstDate)} – {fullDateFormat.format(lastDate)}</p>
    <p className="chart-note">Ultimul curs salvat în fiecare zi. Scara verticală este ajustată la valorile afișate.</p>
    {bnrWarning}
  </div>
}
