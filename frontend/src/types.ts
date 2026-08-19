export type DecimalLike = string | number

export type Offer = {
  provider: string
  provider_name: string
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
}

export type Comparison = {
  from: string
  to: string
  input_amount: DecimalLike
  bnr_output: DecimalLike
  bnr_rate: DecimalLike
  bnr_fetched_at: string
  offers: Offer[]
  missing_sources: string[]
}

export type History = {
  provider: string
  currency: string
  side: 'buy' | 'sell'
  points: { date: string; provider_rate: DecimalLike; bnr_rate: DecimalLike }[]
}

