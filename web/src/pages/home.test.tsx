import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'

// Mock navigate
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual('react-router-dom')
  return { ...actual, useNavigate: () => vi.fn() }
})

// Mock auth context (Header uses useAuth)
vi.mock('@/auth/AuthContext', () => ({
  useAuth: () => ({
    accessToken: null,
    login: vi.fn(),
    logout: vi.fn(),
    loginError: null,
  }),
}))

// Mock theme (Header uses ThemeToggle which uses useTheme)
vi.mock('@/components/providers', () => ({
  useTheme: () => ({ theme: 'light', setTheme: vi.fn() }),
}))

// Mock API client
const mockGET = vi.fn()
vi.mock('@/lib/api-client', () => ({
  getPublicClient: () => ({ GET: mockGET }),
  getAuthenticatedClient: () => ({ GET: mockGET }),
}))

import HomePage from './home'

function renderHome() {
  const qc = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter>
        <HomePage />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

describe('HomePage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockGET.mockResolvedValue({ data: { items: [] } })
  })

  it('renders the hero heading', () => {
    renderHome()
    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent(/discover, publish and integrate mcp servers/i)
  })

  it('renders the hero description', () => {
    renderHome()
    expect(screen.getByText(/one catalog for the tools your models can call/i)).toBeInTheDocument()
  })

  it('exposes an MCP Servers entry point via the header nav', () => {
    renderHome()
    // Header nav link; the hero does not duplicate this CTA.
    const links = screen.getAllByRole('link', { name: /mcp servers/i })
    expect(links.some((l) => l.getAttribute('href') === '/mcp')).toBe(true)
  })

  it('exposes a Getting Started entry point via the header nav', () => {
    renderHome()
    const links = screen.getAllByRole('link', { name: /getting started/i })
    expect(links.some((l) => l.getAttribute('href') === '/getting-started')).toBe(true)
  })

  it('renders the search input', () => {
    renderHome()
    expect(screen.getByPlaceholderText(/search mcp servers/i)).toBeInTheDocument()
  })

  it('renders the protocol entry tile', () => {
    renderHome()
    expect(screen.getByRole('heading', { name: 'MCP servers' })).toBeInTheDocument()
  })

  it('shows stat placeholders before data loads', () => {
    renderHome()
    // The em-dash placeholders
    const dashes = screen.getAllByText('—')
    expect(dashes.length).toBeGreaterThanOrEqual(2)
  })

  it('renders a "View all" link to the catalog', () => {
    renderHome()
    expect(screen.getByRole('link', { name: /view all/i })).toHaveAttribute('href', '/mcp')
  })

  it('shows empty messages when no entries exist', () => {
    // With items: [] returned, after featured check falls through to recent
    mockGET.mockResolvedValue({ data: { items: [] } })
    renderHome()
    expect(screen.getByRole('heading', { level: 1 })).toBeInTheDocument()
  })
})

type Query = { featured?: boolean; sort?: string; limit?: number }
type Listing = { items: { id: string; name: string }[] }

function entry(id: string, name: string) {
  return {
    id,
    namespace: 'acme',
    slug: id,
    name,
    description: 'desc',
    status: 'published',
    verified: false,
    view_count: 0,
    created_at: '2025-01-01T00:00:00Z',
    updated_at: '2025-01-15T00:00:00Z',
  }
}

// Routes each listing variant (featured / recent / updated) to its own payload.
function primeListings(listings: {
  mcp: Partial<Record<'featured' | 'recent' | 'updated', Listing>>
  stats?: Record<string, number>
}) {
  mockGET.mockImplementation((path: string, init?: { params?: { query?: Query } }) => {
    if (path === '/api/v1/public-stats') return Promise.resolve({ data: listings.stats ?? {} })
    const q = init?.params?.query ?? {}
    const variant = q.featured ? 'featured' : q.sort === 'updated_at_desc' ? 'updated' : 'recent'
    return Promise.resolve({ data: listings.mcp[variant] ?? { items: [] } })
  })
}

describe('HomePage listings', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('shows featured entries when some are featured', async () => {
    primeListings({
      mcp: { featured: { items: [entry('fs', 'Featured Server')] } },
    })
    renderHome()

    expect(await screen.findByRole('heading', { name: 'Featured MCP Servers' })).toBeInTheDocument()
    expect(await screen.findByText('Featured Server')).toBeInTheDocument()
  })

  it('falls back to the most recent entries when nothing is featured', async () => {
    primeListings({
      mcp: { recent: { items: [entry('new', 'Newest Server')] } },
    })
    renderHome()

    expect(await screen.findByText('Newest Server')).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Recent MCP Servers' })).toBeInTheDocument()
  })

  it('says so when the registry is empty', async () => {
    primeListings({ mcp: {} })
    renderHome()

    expect(await screen.findByText('No MCP servers published yet.')).toBeInTheDocument()
  })

  it('switches to recently updated entries on toggle', async () => {
    const user = userEvent.setup()
    primeListings({
      mcp: { featured: { items: [entry('fs', 'Featured Server')] }, updated: { items: [entry('up', 'Updated Server')] } },
    })
    renderHome()
    await screen.findByText('Featured Server')

    await user.click(screen.getByRole('button', { name: 'Recently updated' }))

    expect(await screen.findByText('Updated Server')).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Recently Updated MCP Servers' })).toBeInTheDocument()
    expect(screen.queryByText('Featured Server')).not.toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Featured' }))
    expect(await screen.findByText('Featured Server')).toBeInTheDocument()
  })

  it('renders the public stats with the total weekly growth', async () => {
    primeListings({
      mcp: {},
      stats: {
        mcp_servers: 12,
        publishers: 4,
        new_mcp_servers_this_week: 3,
        new_publishers_this_week: 1,
      },
    })
    renderHome()

    expect(await screen.findByText('12')).toBeInTheDocument()
    expect(screen.getByText('4')).toBeInTheDocument()
    expect(screen.getByText('+4 this week')).toBeInTheDocument()
  })

  it('hides weekly growth when nothing is new', async () => {
    primeListings({
      mcp: {},
      stats: { mcp_servers: 12, publishers: 4, new_mcp_servers_this_week: 0 },
    })
    renderHome()

    expect(await screen.findByText('12')).toBeInTheDocument()
    expect(screen.queryByText(/this week/)).not.toBeInTheDocument()
  })
})
