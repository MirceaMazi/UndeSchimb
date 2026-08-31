import { readdir, readFile, writeFile } from 'node:fs/promises'
import { join, relative, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const frontendDirectory = resolve(fileURLToPath(new URL('..', import.meta.url)))
const outputDirectory = join(frontendDirectory, 'dist')
const token = process.env.CLOUDFLARE_WEB_ANALYTICS_TOKEN?.trim()

if (!token) {
  console.log('Cloudflare Web Analytics: token absent, injection skipped.')
  process.exit(0)
}

if (!/^[A-Za-z0-9_-]+$/.test(token)) {
  throw new Error('CLOUDFLARE_WEB_ANALYTICS_TOKEN contains unsupported characters.')
}

async function findHtmlPages(directory) {
  const entries = await readdir(directory, { withFileTypes: true })
  const nested = await Promise.all(entries.map(async (entry) => {
    const path = join(directory, entry.name)
    if (entry.isDirectory()) return findHtmlPages(path)
    return entry.isFile() && entry.name.endsWith('.html') ? [path] : []
  }))
  return nested.flat()
}

const beacon = [
  '  <script',
  '    type="module"',
  '    src="https://static.cloudflareinsights.com/beacon.min.js"',
  `    data-cf-beacon='${JSON.stringify({ token })}'`,
  '  ></script>',
].join('\n')

const pages = await findHtmlPages(outputDirectory)
if (!pages.length) {
  throw new Error('Cloudflare Web Analytics: no HTML pages found in dist.')
}

let injectedPages = 0
let existingPages = 0

for (const path of pages) {
  const html = await readFile(path, 'utf8')
  const beaconCount = [...html.matchAll(/data-cf-beacon\s*=/gi)].length

  if (beaconCount > 1) {
    throw new Error(`${relative(outputDirectory, path)} contains more than one Cloudflare beacon.`)
  }

  if (beaconCount === 1) {
    existingPages += 1
    continue
  }

  if (!/<\/body>/i.test(html)) {
    throw new Error(`${relative(outputDirectory, path)} does not contain a closing body tag.`)
  }

  const instrumentedHtml = html.replace(/<\/body>/i, `${beacon}\n</body>`)
  await writeFile(path, instrumentedHtml)
  injectedPages += 1
}

console.log(
  `Cloudflare Web Analytics: ${injectedPages} page(s) instrumented, ${existingPages} already instrumented.`,
)
