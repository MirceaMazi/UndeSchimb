import { renderToStaticMarkup, renderToString } from 'react-dom/server'
import App, { type AppProps } from './App'
import LandingRates from './LandingRates'
import type { ComponentProps } from 'react'

export { landingPages } from './LandingRates'
export { normalizeComparison, ageComparison } from './comparison-data'

export function render(props: AppProps = {}) {
  return renderToString(<App {...props} />)
}

export function renderRates(props: ComponentProps<typeof LandingRates>) {
  return renderToStaticMarkup(<LandingRates {...props} />)
}
