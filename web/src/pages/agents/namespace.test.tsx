import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, within } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'

// See `pages/mcp/namespace.test.tsx` for the rationale behind these mocks —
// the two pages are structural mirrors and share the same test harness.
vi.mock('@/auth/AuthContext', () => ({
  useAuth: () => ({ accessToken: null, login: vi.fn(), logout: vi.fn(), loginError: null }),
}))
vi.mock('@/components/providers', () => ({
  useTheme: () => ({ theme: 'light', setTheme: vi.fn() }),
}))

const mockGET = vi.fn()
vi.mock('@/lib/api-client', () => ({
  getPublicClient: () => ({ GET: mockGET }),
}))

import AgentNamespacePage from './namespace'

function renderAt(namespace: string) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={[`/agents/${namespace}`]}>
        <Routes>
          <Route path="/agents/:namespace" element={<AgentNamespacePage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

const publisherPayload = {
  id: '1',
  slug: 'acme',
  name: 'Acme Corp',
  verified: false,
  created_at: '2025-01-01T00:00:00Z',
  updated_at: '2025-01-01T00:00:00Z',
}

function makeAgentRow(slug: string, name: string) {
  return {
    id: `01H${slug}`,
    namespace: 'acme',
    slug,
    name,
    description: 'desc',
    status: 'published',
    verified: false,
    view_count: 0,
    updated_at: '2025-01-15T00:00:00Z',
    created_at: '2025-01-01T00:00:00Z',
    latest_version: {
      version: '1.0.0',
      skills: [],
      endpoint_url: 'https://agent.example.com/a2a',
    },
  }
}

describe('AgentNamespacePage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders publisher header + agent cards when both queries resolve with data', async () => {
    mockGET.mockImplementation((path: string) => {
      if (path.includes('/publishers/')) {
        return Promise.resolve({ data: publisherPayload })
      }
      // /api/v1/agents
      return Promise.resolve({
        data: {
          items: [makeAgentRow('helper', 'Helper Bot'), makeAgentRow('coder', 'Coder Bot')],
          total_count: 2,
        },
      })
    })

    renderAt('acme')

    expect(await screen.findByRole('heading', { name: 'Acme Corp' })).toBeInTheDocument()

    // Breadcrumbs: Home › Agents › acme.
    const nav = screen.getByRole('navigation', { name: /breadcrumb/i })
    expect(nav).toHaveTextContent('Home')
    expect(nav).toHaveTextContent('Agents')
    expect(nav).toHaveTextContent('acme')

    // Both cards rendered.
    expect(screen.getByText('Helper Bot')).toBeInTheDocument()
    expect(screen.getByText('Coder Bot')).toBeInTheDocument()

    // Showing-count line.
    expect(screen.getByText(/Showing 2.*agents?/)).toBeInTheDocument()
  })

  it('renders the empty-state copy when the publisher exists but has zero agents', async () => {
    mockGET.mockImplementation((path: string) => {
      if (path.includes('/publishers/')) {
        return Promise.resolve({ data: publisherPayload })
      }
      return Promise.resolve({ data: { items: [], total_count: 0 } })
    })

    renderAt('acme')

    expect(await screen.findByRole('heading', { name: 'Acme Corp' })).toBeInTheDocument()
    expect(screen.getByText(/no agents yet/i)).toBeInTheDocument()
    const browseAll = screen.getByRole('link', { name: /browse all agents/i })
    expect(browseAll).toHaveAttribute('href', '/agents')
  })

  it('renders the namespace-not-found state when the publisher 404s', async () => {
    mockGET.mockImplementation((path: string) => {
      if (path.includes('/publishers/')) {
        return Promise.resolve({ data: undefined })
      }
      return Promise.resolve({ data: { items: [], total_count: 0 } })
    })

    renderAt('ghost-ns')

    expect(await screen.findByText(/namespace not found/i)).toBeInTheDocument()
    expect(screen.getByText(/ghost-ns/)).toBeInTheDocument()
    expect(screen.queryByRole('navigation', { name: /breadcrumb/i })).not.toBeInTheDocument()
  })

  it('breadcrumb links point at flat list routes', async () => {
    mockGET.mockImplementation((path: string) => {
      if (path.includes('/publishers/')) {
        return Promise.resolve({ data: publisherPayload })
      }
      return Promise.resolve({ data: { items: [], total_count: 0 } })
    })

    renderAt('acme')

    // Scope to the breadcrumb nav — the site Header also has top-level nav
    // links that would collide with a bare `getByRole` query.
    await screen.findByRole('heading', { name: 'Acme Corp' })
    const nav = screen.getByRole('navigation', { name: /breadcrumb/i })
    const home = within(nav).getByRole('link', { name: 'Home' })
    expect(home).toHaveAttribute('href', '/')
    const agents = within(nav).getByRole('link', { name: 'Agents' })
    expect(agents).toHaveAttribute('href', '/agents')
  })

  it('header links to the full publisher profile', async () => {
    mockGET.mockImplementation((path: string) => {
      if (path.includes('/publishers/')) {
        return Promise.resolve({ data: publisherPayload })
      }
      return Promise.resolve({ data: { items: [], total_count: 0 } })
    })

    renderAt('acme')

    const profile = await screen.findByRole('link', { name: /view publisher profile/i })
    expect(profile).toHaveAttribute('href', '/publishers/acme')
  })
})
