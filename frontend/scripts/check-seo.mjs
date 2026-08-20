import { readdir, readFile } from 'node:fs/promises'
import { join, relative, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const frontendDirectory = resolve(fileURLToPath(new URL('..', import.meta.url)))
const publicDirectory = join(frontendDirectory, 'public')
const productionOrigin = 'https://undeschimb.onrender.com'

async function findStaticPages(directory) {
  const entries = await readdir(directory, { withFileTypes: true })
  const nested = await Promise.all(entries.map(async (entry) => {
    const path = join(directory, entry.name)
    if (entry.isDirectory()) return findStaticPages(path)
    return entry.name === 'index.html' ? [path] : []
  }))
  return nested.flat()
}

function match(html, pattern) {
  return html.match(pattern)?.[1]?.trim() ?? ''
}

function requireValue(value, message, errors) {
  if (!value) errors.push(message)
}

const sourcePages = [join(frontendDirectory, 'index.html'), ...(await findStaticPages(publicDirectory))]
const errors = []
const warnings = []
const canonicals = new Map()
const titles = new Map()
const descriptions = new Map()
const pageRecords = []

for (const path of sourcePages) {
  const html = await readFile(path, 'utf8')
  const label = relative(frontendDirectory, path).replaceAll('\\', '/')
  const title = match(html, /<title>([\s\S]*?)<\/title>/i)
  const description = match(html, /<meta\s+name=["']description["']\s+content=["']([^"']+)["']\s*\/?>/i)
  const canonical = match(html, /<link\s+rel=["']canonical["']\s+href=["']([^"']+)["']\s*\/?>/i)
  const ogTitle = match(html, /<meta\s+property=["']og:title["']\s+content=["']([^"']+)["']\s*\/?>/i)
  const ogDescription = match(html, /<meta\s+property=["']og:description["']\s+content=["']([^"']+)["']\s*\/?>/i)
  const ogUrl = match(html, /<meta\s+property=["']og:url["']\s+content=["']([^"']+)["']\s*\/?>/i)
  const twitterCard = match(html, /<meta\s+name=["']twitter:card["']\s+content=["']([^"']+)["']\s*\/?>/i)

  requireValue(title, `${label}: lipsește <title>`, errors)
  requireValue(description, `${label}: lipsește meta description`, errors)
  requireValue(canonical, `${label}: lipsește canonical`, errors)
  requireValue(ogTitle, `${label}: lipsește og:title`, errors)
  requireValue(ogDescription, `${label}: lipsește og:description`, errors)
  requireValue(ogUrl, `${label}: lipsește og:url`, errors)
  requireValue(twitterCard, `${label}: lipsește twitter:card`, errors)

  if (!/<html\s+lang=["']ro["']/i.test(html)) errors.push(`${label}: limba paginii nu este ro`)
  if (path !== sourcePages[0] && !/<h1[\s>]/i.test(html)) errors.push(`${label}: lipsește H1`)
  if (canonical && !canonical.startsWith(`${productionOrigin}/`)) errors.push(`${label}: canonical nu folosește domeniul de producție`)
  if (canonical && ogUrl && canonical !== ogUrl) errors.push(`${label}: og:url diferă de canonical`)
  if (title.length > 70) warnings.push(`${label}: titlul are ${title.length} caractere`)
  if (description.length < 90 || description.length > 180) warnings.push(`${label}: descrierea are ${description.length} caractere`)

  for (const script of html.matchAll(/<script\s+type=["']application\/ld\+json["']>([\s\S]*?)<\/script>/gi)) {
    try {
      JSON.parse(script[1])
    } catch (error) {
      errors.push(`${label}: JSON-LD invalid (${error.message})`)
    }
  }

  if (canonical) {
    if (canonicals.has(canonical)) errors.push(`${label}: canonical duplicat cu ${canonicals.get(canonical)}`)
    canonicals.set(canonical, label)
  }
  if (title) {
    if (titles.has(title)) errors.push(`${label}: titlu duplicat cu ${titles.get(title)}`)
    titles.set(title, label)
  }
  if (description) {
    if (descriptions.has(description)) errors.push(`${label}: descriere duplicată cu ${descriptions.get(description)}`)
    descriptions.set(description, label)
  }
  pageRecords.push({ label, html })
}

const appSource = await readFile(join(frontendDirectory, 'src', 'App.tsx'), 'utf8')
if (!/<h1[\s>]/.test(appSource)) errors.push('src/App.tsx: lipsește H1-ul paginii principale')

const sitemap = await readFile(join(publicDirectory, 'sitemap.xml'), 'utf8')
const sitemapUrls = new Set([...sitemap.matchAll(/<loc>([^<]+)<\/loc>/g)].map((entry) => entry[1]))
for (const canonical of canonicals.keys()) {
  if (!sitemapUrls.has(canonical)) errors.push(`sitemap.xml: lipsește ${canonical}`)
}
for (const url of sitemapUrls) {
  if (!canonicals.has(url)) errors.push(`sitemap.xml: URL fără pagină canonicală ${url}`)
}

const availableRoutes = new Set([...canonicals.keys()].map((url) => new URL(url).pathname))
for (const { label, html } of pageRecords) {
  for (const link of html.matchAll(/<a\s+[^>]*href=["']([^"']+)["']/gi)) {
    const href = link[1]
    if (!href.startsWith('/') || href.startsWith('//')) continue
    const pathname = new URL(href, productionOrigin).pathname
    if (pathname === '/' || pathname.startsWith('/assets/') || /\.[a-z0-9]+$/i.test(pathname)) continue
    if (!availableRoutes.has(pathname)) errors.push(`${label}: link intern fără pagină ${pathname}`)
  }
}

for (const warning of warnings) console.warn(`SEO warning: ${warning}`)
if (errors.length) {
  for (const error of errors) console.error(`SEO error: ${error}`)
  process.exitCode = 1
} else {
  console.log(`SEO check passed: ${sourcePages.length} pages, ${sitemapUrls.size} sitemap URLs, valid JSON-LD.`)
}
