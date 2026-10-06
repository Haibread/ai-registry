import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor, within } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'

vi.mock('@/auth/AuthContext', () => ({
  useAuth: () => ({ accessToken: 'test-token' }),
}))

const mockGET = vi.fn()
vi.mock('@/lib/api-client', () => ({
  useAuthClient: () => ({ GET: mockGET }),
}))

// The dashboard routes by publisher scope. Default to the Server-Admin "All
// publishers" scope so these tests exercise the global dashboard; individual
// tests override mockPub to hit the other branches.
let mockPub = {
  publishers: [] as unknown[],
  isServerAdmin: true,
  currentSlug: null as string | null,
  current: null as unknown,
  setCurrent: vi.fn(),
  isLoading: false,
}
vi.mock('@/auth/PublisherContext', () => ({
  usePublisher: () => mockPub,
}))

import AdminDashboard from './dashboard'

function renderPage() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter>
        <AdminDashboard />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

const sampleStats = {
  mcp_servers: 12,
  publishers: 3,
  mcp_status_breakdown: { draft: 2, published: 9, deprecated: 1 },
}

const sampleMcp = [
  {
    id: '01HMCP1',
    name: 'Example MCP',
    namespace: 'acme',
    slug: 'example-mcp',
    status: 'published',
    updated_at: '2026-04-10T10:00:00Z',
  },
]

describe('AdminDashboard', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockPub = {
      publishers: [],
      isServerAdmin: true,
      currentSlug: null,
      current: null,
      setCurrent: vi.fn(),
      isLoading: false,
    }
    mockGET.mockImplementation((path: string) => {
      if (path === '/api/v1/stats') return Promise.resolve({ data: sampleStats })
      if (path === '/api/v1/mcp/servers') return Promise.resolve({ data: { items: sampleMcp } })
      return Promise.resolve({ data: {} })
    })
  })

  it('renders the Dashboard heading', () => {
    renderPage()
    expect(screen.getByRole('heading', { name: /dashboard/i })).toBeInTheDocument()
  })

  it('fetches stats and recent MCP servers on mount', async () => {
    renderPage()
    await waitFor(() => {
      expect(mockGET).toHaveBeenCalledWith('/api/v1/stats')
    })
    expect(mockGET).toHaveBeenCalledWith('/api/v1/mcp/servers', {
      params: { query: { limit: 5 } },
    })
  })

  it('renders stat tile counts from the API response', async () => {
    renderPage()
    expect(await screen.findByText('12')).toBeInTheDocument() // mcp_servers
    expect(screen.getByText('3')).toBeInTheDocument() // publishers
  })

  it('renders no quick-action block — the New buttons live on the list pages', () => {
    renderPage()
    // The stat cards already link to each list page, which carries its own
    // New button, so there is no separate Quick Actions block.
    expect(screen.queryByText(/quick actions/i)).not.toBeInTheDocument()
    expect(screen.queryByRole('link', { name: /new publisher/i })).not.toBeInTheDocument()
  })

  it('renders recent MCP servers with links to detail pages', async () => {
    renderPage()
    expect(await screen.findByText('Example MCP')).toBeInTheDocument()
    const mcpLink = screen
      .getAllByRole('link')
      .find((a) => a.getAttribute('href') === '/admin/mcp/acme/example-mcp')
    expect(mcpLink).toBeTruthy()
  })

  it('puts pending reviews and open reports first', async () => {
    mockGET.mockImplementation((path: string) => {
      if (path === '/api/v1/stats') return Promise.resolve({ data: sampleStats })
      if (path === '/api/v1/review-queue') return Promise.resolve({ data: { items: [{}, {}] } })
      if (path === '/api/v1/reports') return Promise.resolve({ data: { items: [{}] } })
      return Promise.resolve({ data: { items: [] } })
    })
    renderPage()
    const strip = await screen.findByRole('region', { name: 'Needs you' })
    expect(within(strip).getByRole('link', { name: /2\s*awaiting your review/i })).toHaveAttribute('href', '/admin/review')
    expect(within(strip).getByRole('link', { name: /1\s*open report/i })).toHaveAttribute('href', '/admin/reports')
    expect(mockGET).toHaveBeenCalledWith('/api/v1/reports', { params: { query: { status: 'pending' } } })
  })

  it('shows an error alert when stats fail to load', async () => {
    mockGET.mockImplementation((path: string) => {
      if (path === '/api/v1/stats') return Promise.reject(new Error('boom'))
      return Promise.resolve({ data: { items: [] } })
    })
    renderPage()
    expect(await screen.findByTestId('stats-error')).toBeInTheDocument()
  })

  it('shows a no-publishers empty state for a member with no grants', () => {
    mockPub = {
      publishers: [],
      isServerAdmin: false,
      currentSlug: null,
      current: null,
      setCurrent: vi.fn(),
      isLoading: false,
    }
    renderPage()
    expect(screen.getByRole('heading', { name: /no publishers yet/i })).toBeInTheDocument()
    // The global dashboard's quick actions must not render in this scope.
    expect(screen.queryByText(/quick actions/i)).not.toBeInTheDocument()
  })
})
