import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'

import AdminApiKeys from './api-keys'

function renderPage() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter>
        <AdminApiKeys />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

describe('AdminApiKeys', () => {
  it('renders the page heading', () => {
    renderPage()
    expect(screen.getByRole('heading', { name: /api keys/i })).toBeInTheDocument()
  })

  it('renders the coming soon card title', () => {
    renderPage()
    expect(screen.getByText(/coming soon/i)).toBeInTheDocument()
  })

  it('explains that hashed API keys are planned for Phase 5', () => {
    renderPage()
    expect(screen.getByText(/phase 5/i)).toBeInTheDocument()
    expect(screen.getByText(/hashed api keys/i)).toBeInTheDocument()
  })

  it('points users at their Keycloak access token in the meantime', () => {
    renderPage()
    expect(screen.getByText(/keycloak access token/i)).toBeInTheDocument()
  })

  // TODO: when Phase 5 ships, replace with tests for the create-key form
  // and one-time secret display (mock POST /api/v1/api-keys).
  it.skip('creating a new key shows the one-time secret display', () => {})
})
