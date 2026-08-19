import type { Comparison, History } from './types'

const baseURL = import.meta.env.VITE_API_URL ?? 'http://localhost:8080/api/v1'

async function request<T>(path: string): Promise<T> {
  const response = await fetch(`${baseURL}${path}`)
  const payload = await response.json().catch(() => ({}))
  if (!response.ok) {
    throw new Error(payload.error ?? 'Nu am putut încărca datele.')
  }
  return payload as T
}

export async function getComparison(from: string, to: string, amount: string) {
  const query = new URLSearchParams({ from, to, amount })
  const comparison = await request<Comparison>(`/comparison?${query}`)
  return {
    ...comparison,
    offers: (comparison.offers ?? []).map((offer) => ({
      ...offer,
      conditions: offer.conditions ?? [],
      location_policy: offer.location_policy ?? 'not_applicable',
      location_label: offer.location_label ?? '',
      location_note: offer.location_note ?? '',
    })),
    provider_notices: (comparison.provider_notices ?? []).map((notice) => ({ ...notice, conditions: notice.conditions ?? [] })),
    missing_sources: comparison.missing_sources ?? [],
  }
}

export function getHistory(provider: string, currency: string, side: 'buy' | 'sell', period: number) {
  const query = new URLSearchParams({ provider, currency, side, period: String(period) })
  return request<History>(`/history?${query}`)
}
