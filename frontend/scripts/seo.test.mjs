import assert from 'node:assert/strict'
import { after, test } from 'node:test'
import { createServer as createHTTPServer } from 'node:http'
import { readFile, mkdtemp, mkdir, writeFile, rm } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { createFrontendServer } from '../server/index.mjs'
import { createSnapshotCache, homeRequest } from '../server/snapshots.mjs'
import { serializeSnapshot } from '../server/render-page.mjs'

const frontendDirectory = fileURLToPath(new URL('..', import.meta.url))
const renderer = await import('../dist-ssr/entry-server.js')
let now = Date.parse('2026-09-27T10:00:00Z')
let unavailable = false
let version = 0
let calls = 0

function comparison(request = homeRequest) {
  const currency = request.from === 'RON' ? request.to : request.from
  const rate = { EUR: 5, USD: 4.5, GBP: 6, CHF: 5.5 }[currency] + version / 10
  const amount = Number(request.amount)
  const output = request.from === 'RON' ? amount / rate : amount * rate
  const timestamp = new Date(now).toISOString()
  const offer = (provider, provider_name, category, extra = {}) => ({
    provider, provider_name, category, offer_type: 'standard', effective_rate: String(rate),
    output_amount: String(output), difference_from_bnr: '0', difference_from_bnr_ron: '0', difference_percent: '0',
    fee_percent: '0', source_url: 'https://example.com/' + provider, fetched_at: timestamp, effective_at: timestamp,
    stale: false, indicative: false, conditional: false, active_now: true, request_eligible: true, conditions: [],
    location_policy: 'not_applicable', location_label: '', location_note: '', ...extra,
  })
  return {
    from: request.from, to: request.to, input_amount: request.amount, bnr_output: String(output), bnr_rate: String(rate),
    bnr_fetched_at: timestamp,
    offers: [
      offer('bcr', 'BCR test', 'banks'),
      offer('xtb', 'XTB test', 'brokers', { indicative: true, offer_type: 'indicative', fee_percent: '0.5' }),
      offer('tavex', 'Tavex test', 'physical_exchanges', { offer_type: 'cash', location_label: 'București', conditions: ['Numai numerar'] }),
      offer('luxor_bucharest', 'Luxor București test', 'physical_exchanges', { offer_type: 'cash', conditions: ['Peste 500 EUR'] }),
      offer('arad', 'Arad test', 'physical_exchanges', { offer_type: 'cash' }),
      offer('ing_preferential', 'ING preferențial test', 'banks', { offer_type: 'preferential', conditional: true }),
      offer('raiffeisen_smart_hour', 'Smart Hour test', 'banks', { offer_type: 'special', active_now: false }),
    ],
    provider_notices: [{ provider: 'ing_preferential', provider_name: 'ING', category: 'banks', kind: 'conditional', title: 'Cu eligibilitate', description: 'Verifică pachetul.', source_url: 'https://example.com/ing', active_now: true, request_eligible: true, conditions: ['Limită lunară'] }],
    missing_sources: [],
  }
}

const api = createHTTPServer((request, response) => {
  calls++
  response.setHeader('Content-Type', 'application/json')
  if (unavailable) { response.writeHead(503); return response.end('{"error":"unavailable"}') }
  const url = new URL(request.url, 'http://localhost')
  if (url.pathname === '/api/v1/history') return response.end('{"points":[]}')
  response.end(JSON.stringify(comparison(Object.fromEntries(url.searchParams))))
})
await new Promise((resolve) => api.listen(0, '127.0.0.1', resolve))
const apiURL = 'http://127.0.0.1:' + api.address().port + '/api/v1'
const directory = await mkdtemp(join(tmpdir(), 'undeschimb-seo-'))
await writeFile(join(directory, 'index.html'), await readFile(join(frontendDirectory, 'index.html')))
for (const route of Object.keys(renderer.landingPages)) {
  await mkdir(join(directory, route.slice(1)), { recursive: true })
  await writeFile(join(directory, route.slice(1), 'index.html'), await readFile(join(frontendDirectory, 'public', route.slice(1), 'index.html')))
}
const { server } = await createFrontendServer({ apiURL, directory, now: () => now })
await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve))
const origin = 'http://127.0.0.1:' + server.address().port
const htmlFor = async (path) => (await fetch(origin + path)).text()
const tables = (html) => [...html.matchAll(/<tbody>([\s\S]*?)<\/tbody>/g)].map((match) => match[1]).join('')

after(async () => {
  await Promise.all([new Promise((resolve) => server.close(resolve)), new Promise((resolve) => api.close(resolve))])
  const cleanupPath = resolve(directory)
  if (!cleanupPath.startsWith(join(resolve(tmpdir()), 'undeschimb-seo-'))) throw new Error('Unexpected test cleanup path')
  await rm(cleanupPath, { recursive: true, force: true })
})

test('homepage contains usable rates in the HTTP response before JavaScript runs', async () => {
  const html = await htmlFor('/')
  assert.match(html, /<table>/)
  assert.match(html, /BCR test/)
  assert.doesNotMatch(html, /Se încarcă sursele disponibile/)
  const embedded = JSON.parse(html.match(/id="comparison-data" type="application\/json">([^<]*)<\/script>/)[1])
  assert.equal(embedded.initialComparison.input_amount, '1000')
  assert.equal(embedded.initialComparison.bnr_rate, '5')
  assert.equal(embedded.initialTime, now)
})

test('all currency landing pages contain both directions, actual amounts and Romanian metadata', async () => {
  for (const currency of ['EUR', 'USD', 'GBP', 'CHF']) {
    const html = await htmlFor('/curs-' + currency.toLowerCase() + '-ron/')
    assert.equal((html.match(/class="rate-table"/g) ?? []).length, 2)
    assert.match(html, /5\.000,00 RON/)
    assert.ok(html.includes('1.000,00 ' + currency))
    assert.ok(html.includes('Cumperi ' + currency))
    assert.ok(html.includes('Vinzi ' + currency))
    assert.match(html, /Sursa oficială/)
    assert.match(html, /2026-09-27T10:00:00.000Z/)
    assert.match(html, /cumpărare|Compară|comparație/)
    assert.doesNotMatch(html, /Compar\?|cump\?rare|v\?nzare/)
  }
})

test('landing tables respect provider categories and keep special offers out of the standard comparison', async () => {
  const banks = await htmlFor('/curs-valutar-banci/')
  assert.match(tables(banks), /BCR test/)
  assert.doesNotMatch(tables(banks), /XTB test|Tavex test|ING preferențial test|Smart Hour test/)
  assert.match(banks, /Limită lunară/)
  const brokers = tables(await htmlFor('/curs-valutar-brokeri/'))
  assert.match(brokers, /XTB test/)
  assert.match(brokers, /Estimare indicativă/)
  assert.doesNotMatch(brokers, /BCR test|Tavex test/)
  const cash = await htmlFor('/case-schimb-valutar-bucuresti/')
  assert.match(tables(cash), /Tavex test/)
  assert.match(tables(cash), /Luxor București test/)
  assert.doesNotMatch(tables(cash), /Arad test|BCR test|XTB test/)
  assert.match(cash, /General Paul Teodorescu 4/)
  assert.match(cash, /https:\/\/www.luxor-exchange.ro\/bucuresti/)
  assert.match(cash, /category=physical_exchanges/)
})

test('cached requests are reused and successful refreshes update the HTML without rebuilding', async () => {
  const before = calls
  await htmlFor('/curs-eur-ron/')
  await htmlFor('/cel-mai-bun-curs-valutar/')
  assert.equal(calls, before)
  version = 1
  now += 61_000
  const html = await htmlFor('/curs-eur-ron/')
  assert.match(html, /5,1000/)
  assert.equal(calls, before + 2)
})

test('an API outage preserves real values and original times with an expired warning', async () => {
  const oldTimestamp = new Date(now).toISOString()
  unavailable = true
  now += 40 * 60_000
  const html = await htmlFor('/curs-eur-ron/')
  assert.match(html, /Actualizarea comparației a eșuat/)
  assert.match(html, /Date expirate/)
  assert.match(html, /5,1000/)
  assert.ok(html.includes(oldTimestamp))
  assert.ok(!html.includes('dateTime="' + new Date(now).toISOString() + '"'))
  const home = await htmlFor('/')
  assert.doesNotMatch(home, /Cea mai bună ofertă disponibilă/)
  unavailable = false
})

test('missing rates have an explicit empty state, and invalid API payloads are rejected', async () => {
  const empty = { comparison: null, checkedAt: null, failed: true }
  const html = renderer.renderRates({ page: renderer.landingPages['/curs-eur-ron/'], buying: empty, selling: empty, now })
  assert.match(html, /Cursurile nu sunt disponibile momentan/)
  assert.doesNotMatch(html, /<tbody>|0,0000/)
  assert.throws(() => renderer.normalizeComparison({ offers: [] }), /date valide/)
  const invalid = comparison()
  invalid.offers[0].effective_rate = 'NaN'
  assert.throws(() => renderer.normalizeComparison(invalid), /date valide/)
})

test('embedded snapshots cannot terminate their script element and source URLs are sanitized', () => {
  const input = { name: '</script><script>alert(1)</script>', unicode: '\u2028' }
  const serialized = serializeSnapshot(input)
  assert.doesNotMatch(serialized, /<\/script>/)
  assert.deepEqual(JSON.parse(serialized), input)
  const payload = comparison()
  payload.offers[0].source_url = 'javascript:alert(1)'
  payload.offers[0].provider_name = '<script>alert(1)</script>'
  const normalized = renderer.normalizeComparison(payload)
  const snapshot = { comparison: normalized, checkedAt: new Date(now).toISOString(), failed: false }
  const html = renderer.renderRates({ page: renderer.landingPages['/curs-eur-ron/'], buying: snapshot, selling: snapshot, now })
  assert.doesNotMatch(html, /javascript:|<script>/)
  assert.match(html, /&lt;script&gt;/)
})

test('concurrent snapshot requests share a fetch, and mismatched responses never replace good data', async () => {
  let count = 0
  let clock = now
  const cache = createSnapshotCache({ apiURL, normalize: renderer.normalizeComparison, now: () => clock, fetchImpl: async () => {
    count++
    await new Promise((resolve) => setTimeout(resolve, 10))
    const payload = comparison()
    if (count > 1) payload.to = 'USD'
    return new Response(JSON.stringify(payload))
  } })
  const [first, second] = await Promise.all([cache.get(homeRequest), cache.get(homeRequest)])
  assert.equal(count, 1)
  assert.equal(first.comparison.to, 'EUR')
  assert.deepEqual(first, second)
  clock += 61_000
  const failed = await cache.get(homeRequest)
  assert.equal(failed.failed, true)
  assert.equal(failed.comparison.to, 'EUR')
  assert.equal(failed.checkedAt, first.checkedAt)
})

test('canonical routes redirect, unknown routes return 404, and the API proxy preserves the requested scenario', async () => {
  const redirect = await fetch(origin + '/curs-eur-ron', { redirect: 'manual' })
  assert.equal(redirect.status, 308)
  assert.equal(redirect.headers.get('location'), '/curs-eur-ron/')
  assert.equal((await fetch(origin + '/missing-page')).status, 404)
  assert.equal((await fetch(origin + '/%2e%2e%5cpackage.json')).status, 400)
  const response = await fetch(origin + '/api/v1/comparison?from=USD&to=RON&amount=2500')
  assert.equal(response.status, 200)
  const payload = await response.json()
  assert.equal(payload.from, 'USD')
  assert.equal(payload.to, 'RON')
  assert.equal(payload.input_amount, '2500')
  assert.equal((await fetch(origin + '/curs-eur-ron/', { method: 'HEAD' })).status, 200)
})

test('calculator links initialize their amount, currency, direction and provider category', () => {
  const previous = globalThis.window
  globalThis.window = {
    location: { search: '?currency=USD&direction=fx-to-ron&amount=5000&category=physical_exchanges' },
    localStorage: { getItem: () => 'light' },
  }
  try {
    const html = renderer.render({ initialComparison: comparison({ from: 'USD', to: 'RON', amount: '5000' }), initialTime: now })
    assert.match(html, /value="5000"/)
    assert.match(html, /value="USD" selected=""/)
    assert.match(html, /Tavex test/)
    assert.doesNotMatch(tables(html), /BCR test/)
    assert.match(html, /5\.000,00 USD/)
  } finally {
    if (previous === undefined) delete globalThis.window
    else globalThis.window = previous
  }
})
