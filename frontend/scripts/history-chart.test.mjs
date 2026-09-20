import assert from 'node:assert/strict'
import { after, test } from 'node:test'
import { fileURLToPath } from 'node:url'
import { createElement } from 'react'
import { renderToStaticMarkup } from 'react-dom/server'
import { createServer } from 'vite'

const server = await createServer({
  root: fileURLToPath(new URL('..', import.meta.url)),
  server: { middlewareMode: true, watch: null },
})
after(() => server.close())
const { default: HistoryChart } = await server.ssrLoadModule('/src/HistoryChart.tsx')
const point = (date, provider_rate = '5.3', bnr_rate = '5.2') => ({ date, provider_rate, bnr_rate })
function render(points, period = 7, side = 'sell') {
  return renderToStaticMarkup(createElement(HistoryChart, {
    history: { provider: 'bcr', currency: 'EUR', side, points }, providerName: 'BCR', period,
  }))
}

test('shows dated and numbered axes, with real time spacing', () => {
  const html = render([point('2026-09-14'), point('2026-09-15'), point('2026-09-20')])
  assert.match(html, /RON pentru 1 EUR/)
  assert.match(html, /5,\d{4}/)
  assert.match(html, /dateTime="2026-09-14"/)
  assert.match(html, /dateTime="2026-09-20"/)
  const coordinates = html.match(/<polyline points="([^"]+)" class="provider-line"/)[1].split(' ')
  assert.equal(Number(coordinates[0].split(',')[0]), 0)
  assert.equal(Number(coordinates[1].split(',')[0]), 166.67)
  assert.equal(Number(coordinates[2].split(',')[0]), 1000)
})

test('flat rates have finite paths, padding and distinct tick labels', () => {
  const html = render([point('2026-09-19', '5.2', '5.2'), point('2026-09-20', '5.2', '5.2')])
  assert.doesNotMatch(html, /NaN|Infinity/)
  const labels = [...html.matchAll(/style="top:[^"]+">([^<]+)<\/span>/g)].map((match) => match[1])
  assert.ok(labels.length >= 3)
  assert.equal(new Set(labels).size, labels.length)
})

test('a seven-day period without BNR still shows provider history', () => {
  const html = render([point('2026-09-19', '5.3', null), point('2026-09-20', '5.4', null)])
  assert.match(html, /class="provider-line"/)
  assert.match(html, /Lipsesc cotații BNR/)
  assert.doesNotMatch(html, /class="bnr-line"|class="bnr-point"|NaN|Infinity/)
  assert.doesNotMatch(html, />0,0000</)
})

test('missing BNR breaks its line rather than interpolating an invented rate', () => {
  const html = render([
    point('2026-09-16'), point('2026-09-17'), point('2026-09-18', '5.3', null),
    point('2026-09-19'), point('2026-09-20'),
  ])
  assert.equal((html.match(/class="bnr-line"/g) ?? []).length, 2)
})

test('isolated BNR observations remain visible as points', () => {
  const html = render([point('2026-09-19'), point('2026-09-20', '5.3', null)])
  assert.match(html, /class="bnr-point"/)
})

test('empty, invalid and single-observation histories are explicit', () => {
  for (const period of [7, 30, 90]) assert.match(render([], period), new RegExp(`ultimele ${period} zile`))
  assert.match(render(null), /Nu există cotații/)
  assert.match(render([point('bad-date'), point('2026-09-20', '0')]), /Nu există cotații/)
  const html = render([point('2026-09-20', '5.3', null)], 7, 'buy')
  assert.match(html, /curs de cumpărare/)
  assert.match(html, /20\.09\.2026/)
  assert.match(html, /Date lipsă/)
  assert.doesNotMatch(html, /NaN|Infinity/)
})
