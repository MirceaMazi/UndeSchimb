export type DecimalLike = string | number

export type ProviderCategory = 'banks' | 'brokers' | 'physical_exchanges'
export type ProviderTab = ProviderCategory | 'all'
export type OfferType = 'standard' | 'preferential' | 'special' | 'indicative' | 'cash'

export type Offer = {
  provider: string
  provider_name: string
  category: ProviderCategory
  offer_type: OfferType
  effective_rate: DecimalLike
  output_amount: DecimalLike
  difference_from_bnr: DecimalLike
  difference_from_bnr_ron: DecimalLike
  difference_percent: DecimalLike
  fee_percent: DecimalLike
  source_url: string
  fetched_at: string
  effective_at: string
  stale: boolean
  indicative: boolean
  conditional: boolean
  active_now: boolean
  request_eligible: boolean
  conditions: string[]
  location_policy: 'not_applicable' | 'same_all_locations' | 'varies_by_city' | 'single_location'
  location_label: string
  location_note: string
}

export type ProviderNotice = {
  provider: string
  provider_name: string
  category: ProviderCategory
  kind: 'conditional' | 'scheduled' | 'quote_required' | 'ineligible'
  title: string
  description: string
  source_url: string
  active_now: boolean
  request_eligible: boolean
  conditions: string[]
}

export type Comparison = {
  from: string
  to: string
  input_amount: DecimalLike
  bnr_output: DecimalLike
  bnr_rate: DecimalLike
  bnr_fetched_at: string
  offers: Offer[]
  provider_notices: ProviderNotice[]
  missing_sources: string[]
}

export type History = {
  provider: string
  currency: string
  side: 'buy' | 'sell'
  points: { date: string; provider_rate: DecimalLike; bnr_rate: DecimalLike }[]
}
