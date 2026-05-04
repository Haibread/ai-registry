import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, within } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'

// Stub out auth + theme providers the layout chrome depends on — the
// namespace page itself doesn't care, but Header/Footer transitively import
// them.
vi.mock('@/auth/AuthContext', () => ({
  useAuth: () => ({ accessToken: null, login: vi.fn(), logout: vi.fn(), loginError: null }),
}))
vi.mock('@/components/providers', () => ({
  useTheme: () => ({ theme: 'light', setTheme: vi.fn() }),
}))

// Single shared mock for both parallel queries (publisher + server list).
// Individual tests rewrite `mockGET.mockImplementation` to shape responses.
const mockGET = vi.fn()
vi.mock('@/lib/api-client', () => ({
  getPublicClient: () => ({ GET: mockGET }),
}))

import MCPNamespacePage from './namespace'

function renderAt(namespace: string) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={[`/mcp/${namespace}`]}>
        <Routes>
          <Route path="/mcp/:namespace" element={<MCPNamespacePage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

const publisherPayload = {
  id: '1',
  slug: 'acme',
  name: 'Acme Corp',
  verified: true,
  created_at: '2025-01-01T00:00:00Z',
  updated_at: '2025-01-01T00:00:00Z',
}

// Minimal MCPServer shape the card needs to render. `as any` on the mock
// payload keeps the test focused on page-level behaviour without pinning us
// to every schema field the card component reads.
function makeServerRow(slug: string, name: string) {
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
      runtime: 'stdio',
      protocol_version: '2025-03-26',
      packages: [
        {
          registryType: 'npm',
          identifier: `@acme/${slug}`,
          version: '1.0.0',
          transport: { type: 'stdio' },
        },
      ],
    },
  }
}

describe('MCPNamespacePage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders publisher header + server cards when both queries resolve with data', async () => {
    mockGET.mockImplementation((path: string) => {
      if (path.includes('/publishers/')) {
        return Promise.resolve({ data: publisherPayload })
      }
      // /api/v1/mcp/servers
      return Promise.resolve({
        data: {
          items: [makeServerRow('files', 'Files Server'), makeServerRow('web', 'Web Scraper')],
          total_count: 2,
        },
      })
    })

    renderAt('acme')

    // Publisher header.
    expect(await screen.findByRole('heading', { name: 'Acme Corp' })).toBeInTheDocument()
    expect(screen.getByText('Verified')).toBeInTheDocument()

    // Breadcrumbs: Home › MCP Servers › acme.
    const nav = screen.getByRole('navigation', { name: /breadcrumb/i })
    expect(nav).toHaveTextContent('Home')
    expect(nav).toHaveTextContent('MCP Servers')
    expect(nav).toHaveTextContent('acme')

    // Both server cards rendered.
    expect(screen.getByText('Files Server')).toBeInTheDocument()
    expect(screen.getByText('Web Scraper')).toBeInTheDocument()

    // Showing-count line.
    expect(screen.getByText(/Showing 2.*servers?/)).toBeInTheDocument()
  })

  it('renders the empty-state copy when the publisher exists but has zero MCP servers', async () => {
    // Namespace resolves, but the server list comes back empty — a real
    // scenario when a publisher has only agents, or hasn't shipped yet.
    mockGET.mockImplementation((path: string) => {
      if (path.includes('/publishers/')) {
        return Promise.resolve({ data: publisherPayload })
      }
      return Promise.resolve({ data: { items: [], total_count: 0 } })
    })

    renderAt('acme')

    expect(await screen.findByRole('heading', { name: 'Acme Corp' })).toBeInTheDocument()
    expect(screen.getByText(/no mcp servers yet/i)).toBeInTheDocument()
    // The CTA points users back to the flat list so they don't dead-end.
    const browseAll = screen.getByRole('link', { name: /browse all mcp servers/i })
    expect(browseAll).toHaveAttribute('href', '/mcp')
  })

  it('renders the namespace-not-found state when the publisher 404s', async () => {
    // openapi-fetch resolves non-2xx responses to `{ data: undefined }`
    // instead of throwing, so "no publisher" is our canonical 404 signal.
    mockGET.mockImplementation((path: string) => {
      if (path.includes('/publishers/')) {
        return Promise.resolve({ data: undefined })
      }
      return Promise.resolve({ data: { items: [], total_count: 0 } })
    })

    renderAt('ghost-ns')

    expect(await screen.findByText(/namespace not found/i)).toBeInTheDocument()
    // The body should name the bad namespace so the user understands why.
    expect(screen.getByText(/ghost-ns/)).toBeInTheDocument()
    // Publisher-specific chrome (heading, breadcrumbs) must NOT render —
    // we haven't confirmed the namespace exists.
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

    // Scope to the breadcrumb nav — the site Header also renders a global
    // "MCP Servers" top-level link, so a bare `getByRole('link', ...)` hits
    // both. We specifically want the breadcrumb's up-one-level link here.
    await screen.findByRole('heading', { name: 'Acme Corp' })
    const nav = screen.getByRole('navigation', { name: /breadcrumb/i })
    const home = within(nav).getByRole('link', { name: 'Home' })
    expect(home).toHaveAttribute('href', '/')
    const mcp = within(nav).getByRole('link', { name: 'MCP Servers' })
    expect(mcp).toHaveAttribute('href', '/mcp')
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
