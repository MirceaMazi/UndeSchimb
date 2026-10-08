import type { Comparison, History } from './types'
import { normalizeComparison } from './comparison-data'

const baseURL = (import.meta.env.VITE_API_URL || '/api/v1').replace(/\/$/, '')

async function request<T>(path: string, signal?: AbortSignal): Promise<T> {
  const response = await fetch(`${baseURL}${path}`, { signal })
  const payload = await response.json().catch(() => ({}))
  if (!response.ok) {
    throw new Error(payload.error ?? 'Nu am putut încărca datele.')
  }
  return payload as T
}

export async function getComparison(from: string, to: string, amount: string, signal?: AbortSignal) {
  const query = new URLSearchParams({ from, to, amount })
  return normalizeComparison(await request<Comparison>(`/comparison?${query}`, signal))
}

export function getHistory(provider: string, currency: string, side: 'buy' | 'sell', period: number, signal?: AbortSignal) {
  const query = new URLSearchParams({ provider, currency, side, period: String(period) })
  return request<History>(`/history?${query}`, signal)
}
