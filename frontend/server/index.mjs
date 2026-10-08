import { createServer } from 'node:http'
import { readFile, stat } from 'node:fs/promises'
import { extname, join, resolve, sep } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'
import { promisify } from 'node:util'
import { gzip } from 'node:zlib'
import { createSnapshotCache, REFRESH_MS } from './snapshots.mjs'
import { renderPage } from './render-page.mjs'

const compress = promisify(gzip)
const frontendDirectory = fileURLToPath(new URL('..', import.meta.url))
const contentTypes = {
  '.html': 'text/html; charset=utf-8', '.css': 'text/css; charset=utf-8', '.js': 'text/javascript; charset=utf-8',
  '.json': 'application/json; charset=utf-8', '.webmanifest': 'application/manifest+json; charset=utf-8',
  '.xml': 'application/xml; charset=utf-8', '.txt': 'text/plain; charset=utf-8',
  '.png': 'image/png', '.svg': 'image/svg+xml', '.jpg': 'image/jpeg', '.jpeg': 'image/jpeg',
  '.webp': 'image/webp', '.ico': 'image/x-icon', '.woff2': 'font/woff2',
}

export async function createFrontendServer({
  apiURL = process.env.API_URL || process.env.VITE_API_URL || 'http://localhost:8080/api/v1',
  directory = join(frontendDirectory, 'dist'),
  renderer,
  now = Date.now,
  refreshMs = REFRESH_MS,
} = {}) {
  renderer ??= await import(pathToFileURL(join(frontendDirectory, 'dist-ssr', 'entry-server.js')).href)
  const root = resolve(directory)
  const templates = new Map(await Promise.all(['/', ...Object.keys(renderer.landingPages)].map(async (pathname) => [
    pathname, await readFile(join(root, pathname.slice(1), 'index.html'), 'utf8'),
  ])))
  const cache = createSnapshotCache({ apiURL, normalize: renderer.normalizeComparison, now, refreshMs })

  const server = createServer(async (request, response) => {
    async function send(status, body, contentType = 'text/plain; charset=utf-8', cacheControl = 'no-store') {
      let bytes = Buffer.isBuffer(body) ? body : Buffer.from(body)
      response.setHeader('Content-Type', contentType)
      response.setHeader('Cache-Control', cacheControl)
      response.setHeader('X-Content-Type-Options', 'nosniff')
      response.setHeader('Vary', 'Accept-Encoding')
      if (bytes.length > 1024 && /\bgzip\b/.test(request.headers['accept-encoding'] ?? '') && /text|json|xml|javascript/.test(contentType)) {
        bytes = await compress(bytes)
        response.setHeader('Content-Encoding', 'gzip')
      }
      response.writeHead(status, { 'Content-Length': bytes.length })
      response.end(request.method === 'HEAD' ? undefined : bytes)
    }
    function redirect(pathname, search) {
      response.writeHead(308, { Location: pathname + search, 'Cache-Control': 'public, max-age=3600' })
      response.end()
    }
    try {
      if (!['GET', 'HEAD'].includes(request.method)) {
        response.setHeader('Allow', 'GET, HEAD')
        return await send(405, 'Method not allowed')
      }
      const url = new URL(request.url, 'http://localhost')
      let pathname
      try { pathname = decodeURIComponent(url.pathname) } catch { return await send(400, 'Invalid URL') }
      if (pathname.includes('\\') || pathname.includes('\0') || pathname.split('/').includes('..') || pathname.startsWith('//')) return await send(400, 'Invalid URL')
      if (pathname === '/healthz') return await send(200, '{"status":"ok"}', contentTypes['.json'])
      if (['/api/v1/comparison', '/api/v1/history'].includes(pathname)) {
        const endpoint = pathname.slice('/api/v1/'.length)
        try {
          const upstream = await fetch(apiURL.replace(/\/$/, '') + '/' + endpoint + url.search, { signal: AbortSignal.timeout(12_000) })
          return await send(upstream.status, JSON.stringify(await upstream.json()), contentTypes['.json'])
        } catch {
          return await send(502, JSON.stringify({ error: 'Serviciul de cursuri nu este disponibil momentan.' }), contentTypes['.json'])
        }
      }
      if (pathname.endsWith('/index.html')) return redirect(pathname.slice(0, -'index.html'.length), url.search)
      if (!pathname.endsWith('/') && templates.has(pathname + '/')) return redirect(pathname + '/', url.search)
      if (templates.has(pathname)) {
        const html = await renderPage({ template: templates.get(pathname), pathname, cache, renderer, now })
        return await send(200, html, contentTypes['.html'], 'public, max-age=0, must-revalidate')
      }
      let filePath = resolve(root, '.' + pathname)
      if (filePath !== root && !filePath.startsWith(root + sep)) return await send(404, 'Pagina nu a fost găsită.')
      let info
      try { info = await stat(filePath) } catch { return await send(404, 'Pagina nu a fost găsită.') }
      if (info.isDirectory()) {
        if (!pathname.endsWith('/')) return redirect(pathname + '/', url.search)
        filePath = join(filePath, 'index.html')
      }
      const contentType = contentTypes[extname(filePath)]
      if (!contentType) return await send(404, 'Pagina nu a fost găsită.')
      let body
      try { body = await readFile(filePath) } catch { return await send(404, 'Pagina nu a fost găsită.') }
      const cacheControl = pathname.startsWith('/assets/') ? 'public, max-age=31536000, immutable'
        : extname(filePath) === '.html' ? 'public, max-age=0, must-revalidate' : 'public, max-age=3600'
      return await send(200, body, contentType, cacheControl)
    } catch (error) {
      console.error('Page rendering failed:', error.message)
      if (!response.headersSent) await send(500, 'Pagina nu este disponibilă momentan.')
      else response.end()
    }
  })
  const timer = setInterval(() => { void cache.warm() }, refreshMs)
  timer.unref()
  server.on('close', () => clearInterval(timer))
  return { server, cache }
}

if (process.argv[1] && pathToFileURL(resolve(process.argv[1])).href === import.meta.url) {
  const { server, cache } = await createFrontendServer()
  const port = Number(process.env.PORT || 5173)
  server.listen(port, '0.0.0.0', () => {
    console.log('UndeSchimb frontend listening on port ' + port)
    void cache.warm()
  })
  for (const signal of ['SIGINT', 'SIGTERM']) process.on(signal, () => server.close())
}
