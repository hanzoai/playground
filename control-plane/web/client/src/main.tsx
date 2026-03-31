import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './index.css'
import App from './App.tsx'

// After a successful page load, clear the reload-attempt flag so future
// chunk errors can trigger one more reload rather than being silently ignored.
window.addEventListener('load', () => {
  sessionStorage.removeItem('chunkLoadReload')
})

// When a lazy-loaded chunk is missing (stale cached bundle after a deploy)
// do a single hard reload to fetch the latest HTML + assets.
window.addEventListener('unhandledrejection', (event) => {
  const msg: string = (event.reason as Error)?.message ?? ''
  if (
    msg.includes('Failed to fetch dynamically imported module') ||
    msg.includes('Importing a module script failed') ||
    msg.includes('Loading chunk') ||
    msg.includes('Loading CSS chunk')
  ) {
    const RELOAD_KEY = 'chunkLoadReload'
    if (!sessionStorage.getItem(RELOAD_KEY)) {
      sessionStorage.setItem(RELOAD_KEY, '1')
      window.location.reload()
    }
  }
})

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
