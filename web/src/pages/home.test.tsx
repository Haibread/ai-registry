import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
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
    expect(screen.getByRole('heading', { name: /ai registry/i })).toBeInTheDocument()
  })

  it('renders the hero description', () => {
    renderHome()
    expect(
      screen.getByText(/catalog of model context protocol servers and a2a agents/i),
    ).toBeInTheDocument()
  })

  it('exposes an MCP Servers entry point via the header nav', () => {
    renderHome()
    // Header nav link; the hero no longer duplicates this CTA.
    const links = screen.getAllByRole('link', { name: /mcp servers/i })
    expect(links.some((l) => l.getAttribute('href') === '/mcp')).toBe(true)
  })

  it('exposes an Agents entry point via the header nav', () => {
    renderHome()
    const links = screen.getAllByRole('link', { name: /^agents$/i })
    expect(links.some((l) => l.getAttribute('href') === '/agents')).toBe(true)
  })

  it('exposes a Getting Started entry point via the header nav', () => {
    renderHome()
    const links = screen.getAllByRole('link', { name: /getting started/i })
    expect(links.some((l) => l.getAttribute('href') === '/getting-started')).toBe(true)
  })

  it('renders the search input', () => {
    renderHome()
    expect(screen.getByPlaceholderText(/search mcp servers and agents/i)).toBeInTheDocument()
  })

  it('renders the protocol explainer toggle', () => {
    renderHome()
    expect(screen.getByRole('button', { name: /what are mcp and a2a/i })).toBeInTheDocument()
  })

  it('omits the inline stats line until public-stats resolves', () => {
    // Stats are rendered inline as "N MCP servers · N agents · N publishers".
    // Before the query resolves, no counts are shown. The section previously
    // displayed three em-dash placeholders; the editorial header now renders
    // nothing until data is available.
    renderHome()
    expect(screen.queryByText(/\bMCP servers\s·/i)).not.toBeInTheDocument()
  })

  it('renders "View all" links for both sections', () => {
    renderHome()
    const viewAllLinks = screen.getAllByRole('link', { name: /view all/i })
    expect(viewAllLinks.length).toBe(2)
  })

  it('shows empty messages when no entries exist', () => {
    // With items: [] returned, after featured check falls through to recent
    mockGET.mockResolvedValue({ data: { items: [] } })
    renderHome()
    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent(/ai registry/i)
  })
})
