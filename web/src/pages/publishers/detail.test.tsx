import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'

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

import PublisherDetailPage from './detail'

function renderPage(slug = 'acme') {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={[`/publishers/${slug}`]}>
        <Routes>
          <Route path="/publishers/:slug" element={<PublisherDetailPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

describe('PublisherDetailPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockGET.mockImplementation((path: string) => {
      if (path.includes('/publishers/')) {
        return Promise.resolve({
          data: { id: '1', slug: 'acme', name: 'Acme Corp', contact: 'hi@acme.com', verified: true, created_at: '2025-01-01T00:00:00Z', updated_at: '2025-01-01T00:00:00Z' },
        })
      }
      return Promise.resolve({ data: { items: [], total_count: 0 } })
    })
  })

  it('renders publisher name', async () => {
    renderPage()
    expect(await screen.findByText('Acme Corp')).toBeInTheDocument()
  })

  it('renders verified badge', async () => {
    renderPage()
    expect(await screen.findByText('Verified')).toBeInTheDocument()
  })

  it('renders slug in publisher info', async () => {
    renderPage()
    // slug appears as font-mono text below the name
    const slugEls = await screen.findAllByText('acme')
    expect(slugEls.length).toBeGreaterThanOrEqual(1)
  })

  it('renders MCP Servers and Agents section headings', async () => {
    renderPage()
    expect(await screen.findByText('MCP Servers')).toBeInTheDocument()
    expect(screen.getByText('Agents')).toBeInTheDocument()
  })

  it('shows empty messages when no entries', async () => {
    renderPage()
    expect(await screen.findByText(/no mcp servers published/i)).toBeInTheDocument()
    expect(screen.getByText(/no agents published/i)).toBeInTheDocument()
  })

  it('lists the publisher entries with counts and links to their namespaces', async () => {
    mockGET.mockImplementation((path: string) => {
      if (path.includes('/publishers/')) {
        return Promise.resolve({
          data: { id: '1', slug: 'acme', name: 'Acme Corp', verified: false, created_at: '2025-01-01T00:00:00Z', updated_at: '2025-01-01T00:00:00Z' },
        })
      }
      const kind = path.includes('mcp') ? 'Server' : 'Agent'
      return Promise.resolve({
        data: {
          items: [{ id: kind, namespace: 'acme', slug: kind.toLowerCase(), name: `Acme ${kind}`, description: 'desc' }],
          total_count: 3,
        },
      })
    })
    renderPage()

    expect(await screen.findByText('Acme Server')).toBeInTheDocument()
    expect(await screen.findByText('Acme Agent')).toBeInTheDocument()
    expect(screen.getAllByText('(3)')).toHaveLength(2)
    const viewAll = screen.getAllByRole('link', { name: /view all/i }).map((l) => l.getAttribute('href'))
    expect(viewAll).toEqual(['/mcp/acme', '/agents/acme'])
    expect(screen.queryByText('Verified')).not.toBeInTheDocument()
    expect(screen.queryByText('Contact')).not.toBeInTheDocument()
  })

  it('renders the not-found state when the publisher does not exist', async () => {
    mockGET.mockResolvedValue({ data: undefined })
    renderPage('ghost')

    expect(await screen.findByText('Publisher not found')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /back to home/i })).toHaveAttribute('href', '/')
  })
})
