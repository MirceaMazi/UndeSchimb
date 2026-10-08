import { homeRequest } from './snapshots.mjs'

export function serializeSnapshot(value) {
  return JSON.stringify(value).replace(/</g, '\\u003c').replace(/\u2028/g, '\\u2028').replace(/\u2029/g, '\\u2029')
}

function replaceSection(template, name, content) {
  const pattern = new RegExp('<!-- ' + name + ':start -->[\\s\\S]*?<!-- ' + name + ':end -->')
  if (!pattern.test(template)) throw new Error('Missing ' + name + ' rendering markers.')
  return template.replace(pattern, () => '<!-- ' + name + ':start -->' + content + '<!-- ' + name + ':end -->')
}

export async function renderPage({ template, pathname, cache, renderer, now = Date.now }) {
  if (pathname === '/') {
    const snapshot = await cache.get(homeRequest)
    const renderedAt = now()
    const props = {
      initialComparison: snapshot.comparison ? renderer.ageComparison(snapshot.comparison, renderedAt, snapshot.failed) : null,
      initialTime: renderedAt,
      initialError: snapshot.failed ? 'Actualizarea cursurilor nu este disponibilă momentan. Confirmă ultimele valori la furnizor.' : null,
    }
    const content = '<div id="root">' + renderer.render(props) + '</div>'
      + '<script id="comparison-data" type="application/json">' + serializeSnapshot(props) + '</script>'
    return replaceSection(template, 'app', content)
  }
  const page = renderer.landingPages[pathname]
  if (!page) return template
  const [buying, selling] = await Promise.all([
    cache.get({ from: 'RON', to: page.currency, amount: '5000' }),
    cache.get({ from: page.currency, to: 'RON', amount: '1000' }),
  ])
  return replaceSection(template, 'rates', renderer.renderRates({ page, buying, selling, now: now() }))
}
