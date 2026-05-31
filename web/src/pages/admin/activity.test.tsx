import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'

vi.mock('@/auth/AuthContext', () => ({ useAuth: () => ({ accessToken: 't' }) }))

const mockGET = vi.fn()
vi.mock('@/lib/api-client', () => ({ useAuthClient: () => ({ GET: mockGET }) }))

let mockPub: { currentSlug: string | null }
vi.mock('@/auth/PublisherContext', () => ({ usePublisher: () => mockPub }))

import AdminActivity from './activity'

function renderPage() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter>
        <AdminActivity />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

beforeEach(() => {
  vi.clearAllMocks()
  mockPub = { currentSlug: 'acme' }
  mockGET.mockResolvedValue({
    data: {
      items: [
        {
          id: '1',
          action: 'mcp_server_version.published',
          resource_type: 'mcp_server',
          resource_slug: 'weather',
          version: '1.4.0',
          actor_email: 'dana@x.test',
          created_at: new Date().toISOString(),
        },
      ],
      next_cursor: '',
    },
  })
})

describe('AdminActivity', () => {
  it('prompts to select a publisher when none is chosen', () => {
    mockPub = { currentSlug: null }
    renderPage()
    expect(screen.getByText(/select a publisher/i)).toBeInTheDocument()
    expect(mockGET).not.toHaveBeenCalled()
  })

  it('renders the publisher activity feed', async () => {
    renderPage()
    expect(await screen.findByText('weather@1.4.0')).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: /^activity$/i })).toBeInTheDocument()
  })

  it('shows an empty state when there is no activity', async () => {
    mockGET.mockResolvedValue({ data: { items: [], next_cursor: '' } })
    renderPage()
    expect(await screen.findByText(/no activity yet/i)).toBeInTheDocument()
  })
})
