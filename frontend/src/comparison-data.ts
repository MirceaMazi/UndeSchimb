import type { Comparison, Offer } from './types'

export const STALE_AFTER_MS = 30 * 60_000

export function isExpired(value: string, now = Date.now()) {
  const timestamp = Date.parse(value)
  return !Number.isFinite(timestamp) || now - timestamp > STALE_AFTER_MS
}

export function publicSourceURL(value: string) {
  try {
    const url = new URL(value)
    return ['https:', 'http:'].includes(url.protocol) ? url.href : ''
  } catch {
    return ''
  }
}

function validNumber(value: unknown, positive = false) {
  return (typeof value === 'string' && value.trim() !== '' || typeof value === 'number')
    && Number.isFinite(Number(value)) && (!positive || Number(value) > 0)
}

// Reject incomplete responses rather than publishing invented or malformed rates.
export function normalizeComparison(comparison: Comparison): Comparison {
  if (!comparison || !['RON', 'EUR', 'USD', 'GBP', 'CHF'].includes(comparison.from)
    || !['RON', 'EUR', 'USD', 'GBP', 'CHF'].includes(comparison.to)
    || (comparison.from === 'RON') === (comparison.to === 'RON')
    || !validNumber(comparison.input_amount, true) || !validNumber(comparison.bnr_rate, true)
    || !validNumber(comparison.bnr_output, true) || !Number.isFinite(Date.parse(comparison.bnr_fetched_at))
    || comparison.offers != null && !Array.isArray(comparison.offers)
    || comparison.provider_notices != null && !Array.isArray(comparison.provider_notices)) {
    throw new Error('Comparația primită nu conține date valide.')
  }
  const offers = (comparison.offers ?? []).map((offer): Offer => {
    if (!offer || typeof offer.provider !== 'string' || typeof offer.provider_name !== 'string'
      || !['banks', 'brokers', 'physical_exchanges'].includes(offer.category)
      || !validNumber(offer.effective_rate, true) || !validNumber(offer.output_amount, true)
      || !validNumber(offer.fee_percent) || Number(offer.fee_percent) < 0
      || !validNumber(offer.difference_from_bnr_ron) || !validNumber(offer.difference_from_bnr)
      || !validNumber(offer.difference_percent) || !Number.isFinite(Date.parse(offer.fetched_at))) {
      throw new Error('O ofertă primită nu conține date valide.')
    }
    return {
      ...offer,
      source_url: publicSourceURL(offer.source_url),
      active_now: offer.active_now ?? true,
      request_eligible: offer.request_eligible ?? true,
      conditions: (offer.conditions ?? []).filter((condition) => typeof condition === 'string'),
      location_policy: offer.location_policy ?? 'not_applicable',
      location_label: offer.location_label ?? '',
      location_note: offer.location_note ?? '',
    }
  })
  return {
    ...comparison,
    offers,
    provider_notices: (comparison.provider_notices ?? []).map((notice) => ({
      ...notice,
      source_url: publicSourceURL(notice.source_url),
      conditions: (notice.conditions ?? []).filter((condition) => typeof condition === 'string'),
    })),
    missing_sources: comparison.missing_sources ?? [],
  }
}

export function ageComparison(comparison: Comparison, now: number, failed = false): Comparison {
  return {
    ...comparison,
    offers: comparison.offers.map((offer) => ({
      ...offer, stale: failed || offer.stale || isExpired(offer.fetched_at, now),
    })),
  }
}
