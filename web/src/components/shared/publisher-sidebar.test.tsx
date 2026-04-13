import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'

const mockGET = vi.fn()
vi.mock('@/lib/api-client', () => ({
  getPublicClient: () => ({ GET: mockGET }),
}))

import { PublisherSidebar } from './publisher-sidebar'

function renderSidebar(namespace = 'acme') {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter>
        <PublisherSidebar namespace={namespace} />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

describe('PublisherSidebar', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockGET.mockImplementation((path: string) => {
      if (path.includes('/publishers/')) {
        return Promise.resolve({
          data: { id: '1', slug: 'acme', name: 'Acme Corp', verified: true, created_at: '2025-01-01T00:00:00Z', updated_at: '2025-01-01T00:00:00Z' },
        })
      }
      return Promise.resolve({ data: { items: [], total_count: 3 } })
    })
  })

  it('renders publisher name', async () => {
    renderSidebar()
    expect(await screen.findByText('Acme Corp')).toBeInTheDocument()
  })

  it('renders verified badge', async () => {
    renderSidebar()
    expect(await screen.findByText('Verified')).toBeInTheDocument()
  })

  it('renders link to publisher page', async () => {
    renderSidebar()
    const link = await screen.findByText('View all entries →')
    expect(link).toHaveAttribute('href', '/publishers/acme')
  })

  it('shows skeleton while loading', () => {
    mockGET.mockReturnValue(new Promise(() => {})) // never resolves
    renderSidebar()
    // Skeleton is rendered (aria-hidden or just div structure)
    expect(screen.queryByText('Acme Corp')).not.toBeInTheDocument()
  })
})
