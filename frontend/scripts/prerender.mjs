import { readFile, writeFile } from 'node:fs/promises'
import { resolve } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'
import { loadEnv } from 'vite'
import { createSnapshotCache } from '../server/snapshots.mjs'
import { renderPage } from '../server/render-page.mjs'

const frontendDirectory = fileURLToPath(new URL('..', import.meta.url))
const renderer = await import(pathToFileURL(resolve(frontendDirectory, 'dist-ssr', 'entry-server.js')).href)
const env = { ...loadEnv('production', frontendDirectory, ''), ...process.env }
const apiURL = env.API_URL || env.VITE_API_URL
const cache = apiURL && /^https?:\/\//.test(apiURL)
  ? createSnapshotCache({ apiURL, normalize: renderer.normalizeComparison })
  : { get: async () => ({ comparison: null, checkedAt: null, failed: true }) }

await Promise.all(['/', ...Object.keys(renderer.landingPages)].map(async (pathname) => {
  const templatePath = resolve(frontendDirectory, 'dist', pathname.slice(1), 'index.html')
  const template = await readFile(templatePath, 'utf8')
  await writeFile(templatePath, await renderPage({ template, pathname, cache, renderer }))
}))
console.log('Prerendered homepage and ' + Object.keys(renderer.landingPages).length + ' comparison pages. Runtime rendering refreshes their rates.')
