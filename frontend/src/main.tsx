import { StrictMode } from 'react'
import { createRoot, hydrateRoot } from 'react-dom/client'
import App, { type AppProps } from './App'
import './styles.css'

const root = document.getElementById('root')!
let initialProps: AppProps = {}
if (!window.location.search) {
  try {
    initialProps = JSON.parse(document.getElementById('comparison-data')?.textContent ?? '{}')
  } catch {
    // The live calculator can still fetch rates if an embedded snapshot is missing.
  }
}
const app = (
  <StrictMode>
    <App {...initialProps} />
  </StrictMode>
)

if (root.hasChildNodes() && !window.location.search) {
  hydrateRoot(root, app)
} else {
  createRoot(root).render(app)
}
