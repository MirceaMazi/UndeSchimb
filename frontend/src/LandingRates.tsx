import { isExpired } from './comparison-data'
import type { Comparison, Offer, ProviderTab } from './types'

export type RateSnapshot = { comparison: Comparison | null; checkedAt: string | null; failed: boolean }
export type LandingPage = { currency: string; category: ProviderTab; title: string }

export const landingPages: Record<string, LandingPage> = {
  '/curs-eur-ron/': { currency: 'EUR', category: 'all', title: 'Compară cursul euro: cumpărare și vânzare' },
  '/curs-usd-ron/': { currency: 'USD', category: 'all', title: 'Compară cursul dolarului: cumpărare și vânzare' },
  '/curs-gbp-ron/': { currency: 'GBP', category: 'all', title: 'Compară cursul lirei sterline: cumpărare și vânzare' },
  '/curs-chf-ron/': { currency: 'CHF', category: 'all', title: 'Compară cursul francului elvețian: cumpărare și vânzare' },
  '/cel-mai-bun-curs-valutar/': { currency: 'EUR', category: 'all', title: 'Unde primești mai mult pentru suma ta?' },
  '/curs-valutar-banci/': { currency: 'EUR', category: 'banks', title: 'Comparație EUR/RON la băncile din România' },
  '/curs-valutar-brokeri/': { currency: 'EUR', category: 'brokers', title: 'Comparație EUR/RON la brokeri și fintech' },
  '/case-schimb-valutar-bucuresti/': { currency: 'EUR', category: 'physical_exchanges', title: 'Curs euro la casele de schimb din București' },
}

const number = (value: string | number, digits = 2) => new Intl.NumberFormat('ro-RO', {
  minimumFractionDigits: digits, maximumFractionDigits: digits,
}).format(Number(value))

function collectedAt(value: string) {
  return new Intl.DateTimeFormat('ro-RO', {
    timeZone: 'Europe/Bucharest', day: '2-digit', month: '2-digit', year: 'numeric',
    hour: '2-digit', minute: '2-digit',
  }).format(new Date(value))
}

function matchesCategory(offer: { provider: string; category: string }, category: ProviderTab) {
  if (category === 'physical_exchanges') return ['tavex', 'luxor_bucharest'].includes(offer.provider)
  return category === 'all' || offer.category === category
}

function offerStatus(offer: Offer, failed: boolean, now: number) {
  if (!offer.request_eligible) return 'Sumă neeligibilă'
  if (!offer.active_now) return 'Indisponibil acum'
  if (failed || offer.stale || isExpired(offer.fetched_at, now)) return 'Date expirate'
  if (offer.indicative) return 'Estimare indicativă'
  return offer.conditional ? 'Verifică condițiile' : 'Curs publicat'
}

function RateTable({ snapshot, page, selling, now }: {
  snapshot: RateSnapshot; page: LandingPage; selling: boolean; now: number
}) {
  const { comparison, failed } = snapshot
  const direction = selling ? 'fx-to-ron' : 'ron-to-fx'
  const amount = selling ? '1000' : '5000'
  const inputCurrency = selling ? page.currency : 'RON'
  const outputCurrency = selling ? 'RON' : page.currency
  const title = selling ? 'Vinzi ' + page.currency : 'Cumperi ' + page.currency
  const offers = (comparison?.offers ?? []).filter((offer) => matchesCategory(offer, page.category)
    && offer.offer_type !== 'special' && offer.offer_type !== 'preferential')
    .sort((left, right) => Number(right.output_amount) - Number(left.output_amount))
  const notices = (comparison?.provider_notices ?? []).filter((notice) => matchesCategory(notice, page.category))
  const calculatorURL = '/?currency=' + page.currency + '&direction=' + direction + '&amount=' + amount
    + '&category=' + page.category + '#calculator'

  return <article className="rate-scenario">
    <div className="rate-scenario-heading">
      <div><h3>{title}</h3><p>Pentru {number(comparison?.input_amount ?? amount)} {inputCurrency}, primești {outputCurrency}.</p></div>
      <a className="button secondary" href={calculatorURL}>Calculează altă sumă</a>
    </div>
    {!comparison ? <p className="rate-warning" role="status">Cursurile nu sunt disponibile momentan. Încearcă din nou sau deschide calculatorul.</p> : <>
      {failed && <p className="rate-warning" role="status">Actualizarea comparației a eșuat. Mai jos sunt ultimele valori salvate; confirmă cursul la furnizor.</p>}
      <p className="rate-reference">Reper BNR: <strong>{number(comparison.bnr_rate, 4)} RON / {page.currency}</strong>.
        {' '}La acest reper: {number(comparison.bnr_output)} {outputCurrency}. Ultima colectare: <time dateTime={comparison.bnr_fetched_at}>{collectedAt(comparison.bnr_fetched_at)}</time> (ora Bucureștiului).
        {isExpired(comparison.bnr_fetched_at, now) && <strong> Reper BNR expirat.</strong>}
      </p>
      {offers.length ? <div className="rate-table-scroll" role="region" aria-label={title + ': oferte'} tabIndex={0}>
        <table className="rate-table">
          <caption>{title} — cursuri în RON pentru 1 {page.currency}, ordonate după suma rezultată</caption>
          <thead><tr><th scope="col">Furnizor</th><th scope="col">{selling ? 'Cumpărare' : 'Vânzare'}</th><th scope="col">Primești</th><th scope="col">Taxă inclusă</th><th scope="col">Sursă și actualizare</th></tr></thead>
          <tbody>{offers.map((offer) => <tr key={offer.provider}>
            <th scope="row"><strong>{offer.provider_name}</strong>
              {offer.location_label && <small>{offer.location_label}</small>}
              <span className="rate-state">{offerStatus(offer, failed, now)}</span>
              {offer.indicative && <small>Prețul executabil poate diferi.</small>}
              {offer.conditions.length > 0 && <details><summary>Condiții</summary><ul>{offer.conditions.map((condition) => <li key={condition}>{condition}</li>)}</ul>{offer.location_note && <p>{offer.location_note}</p>}</details>}
            </th>
            <td>{number(offer.effective_rate, 4)}</td>
            <td><strong>{number(offer.output_amount)} {outputCurrency}</strong><small>Față de BNR: {number(offer.difference_from_bnr_ron)} RON</small></td>
            <td>{Number(offer.fee_percent) > 0 ? number(offer.fee_percent) + '%' : '0% în calcul'}<small>Verifică taxele contului.</small></td>
            <td>{offer.source_url ? <a href={offer.source_url} rel="noreferrer" target="_blank">Sursa oficială</a> : <span>Sursă indisponibilă</span>}<small><time dateTime={offer.fetched_at}>{collectedAt(offer.fetched_at)}</time></small></td>
          </tr>)}</tbody>
        </table>
      </div> : <p className="rate-warning">Nu avem cursuri pentru această categorie și sumă. Vezi condițiile furnizorilor mai jos.</p>}
      {notices.length > 0 && <details className="rate-conditions"><summary>Oferte condiționate, praguri și limite</summary>
        <ul>{notices.map((notice) => <li key={notice.provider + notice.title}><strong>{notice.provider_name}: {notice.title}.</strong> {notice.description}
          {notice.conditions.length > 0 && <ul>{notice.conditions.map((condition) => <li key={condition}>{condition}</li>)}</ul>}
          {notice.source_url && <> <a href={notice.source_url} rel="noreferrer" target="_blank">Verifică la furnizor</a>.</>}
        </li>)}</ul>
      </details>}
    </>}
  </article>
}

export default function LandingRates({ page, buying, selling, now }: {
  page: LandingPage; buying: RateSnapshot; selling: RateSnapshot; now: number
}) {
  return <section className="live-rates shell" id="cursuri" aria-labelledby="rates-title">
    <p className="eyebrow">CURSURI ȘI SUME COMPARABILE</p>
    <h2 id="rates-title">{page.title}</h2>
    <p className="rates-intro">Două exemple pentru {page.currency}/RON, cu taxele de conversie incluse în calcul. Cursurile sunt publicate de furnizorii urmăriți, iar colectorul le verifică la fiecare 15 minute cât timp serviciul este activ. Verifică ora fiecărei surse și eligibilitatea înainte de schimb.</p>
    <RateTable page={page} snapshot={buying} selling={false} now={now} />
    <RateTable page={page} snapshot={selling} selling now={now} />
    <p className="rate-reference">Cumpărare și vânzare sunt exprimate din perspectiva furnizorului. BNR este un reper, nu o ofertă executabilă. Ofertele preferențiale se pot compara în calculator după activarea condițiilor aplicabile.</p>
  </section>
}
