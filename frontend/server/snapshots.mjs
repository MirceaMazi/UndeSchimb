export const REFRESH_MS = 60_000

export const landingRequests = ['EUR', 'USD', 'GBP', 'CHF'].flatMap((currency) => [
  { from: 'RON', to: currency, amount: '5000' },
  { from: currency, to: 'RON', amount: '1000' },
])
export const homeRequest = { from: 'RON', to: 'EUR', amount: '1000' }

export function createSnapshotCache({ apiURL, normalize, fetchImpl = fetch, now = Date.now, refreshMs = REFRESH_MS, timeoutMs = 8000 }) {
  const entries = new Map()
  const pending = new Map()
  const baseURL = new URL(apiURL.replace(/\/$/, '') + '/')
  if (!['http:', 'https:'].includes(baseURL.protocol)) throw new Error('API_URL must be an HTTP(S) API base URL.')

  async function get(request) {
    const key = new URLSearchParams(request).toString()
    if (pending.has(key)) return pending.get(key)
    const previous = entries.get(key)
    if (previous && now() - previous.attemptedAt < refreshMs) return previous

    const task = (async () => {
      try {
        const url = new URL('comparison?' + key, baseURL)
        const response = await fetchImpl(url, { signal: AbortSignal.timeout(timeoutMs), headers: { Accept: 'application/json' } })
        if (!response.ok) throw new Error('Comparison API returned ' + response.status)
        const comparison = normalize(await response.json())
        if (comparison.from !== request.from || comparison.to !== request.to || Number(comparison.input_amount) !== Number(request.amount)) {
          throw new Error('The comparison does not match the requested currencies and amount.')
        }
        const entry = { comparison, checkedAt: new Date(now()).toISOString(), failed: false, attemptedAt: now() }
        entries.set(key, entry)
        return entry
      } catch {
        // Keep real values and their original timestamps when an upstream refresh fails.
        const entry = { comparison: previous?.comparison ?? null, checkedAt: previous?.checkedAt ?? null, failed: true, attemptedAt: now() }
        entries.set(key, entry)
        return entry
      } finally {
        pending.delete(key)
      }
    })()
    pending.set(key, task)
    return task
  }

  return { get, warm: () => Promise.allSettled([homeRequest, ...landingRequests].map(get)) }
}
