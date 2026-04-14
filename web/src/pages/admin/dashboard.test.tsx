import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'

vi.mock('@/auth/AuthContext', () => ({
  useAuth: () => ({ accessToken: 'test-token' }),
}))

const mockGET = vi.fn()
vi.mock('@/lib/api-client', () => ({
  useAuthClient: () => ({ GET: mockGET }),
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
  agents: 7,
  publishers: 3,
  mcp_status_breakdown: { draft: 2, published: 9, deprecated: 1 },
  agent_status_breakdown: { draft: 1, published: 5, deprecated: 1 },
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

const sampleAgents = [
  {
    id: '01HAGT1',
    name: 'Example Agent',
    namespace: 'acme',
    slug: 'example-agent',
    status: 'draft',
    updated_at: '2026-04-11T10:00:00Z',
  },
]

describe('AdminDashboard', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockGET.mockImplementation((path: string) => {
      if (path === '/api/v1/stats') return Promise.resolve({ data: sampleStats })
      if (path === '/api/v1/mcp/servers') return Promise.resolve({ data: { items: sampleMcp } })
      if (path === '/api/v1/agents') return Promise.resolve({ data: { items: sampleAgents } })
      return Promise.resolve({ data: {} })
    })
  })

  it('renders the Dashboard heading', () => {
    renderPage()
    expect(screen.getByRole('heading', { name: /dashboard/i })).toBeInTheDocument()
  })

  it('fetches stats, recent MCP servers, and recent agents on mount', async () => {
    renderPage()
    await waitFor(() => {
      expect(mockGET).toHaveBeenCalledWith('/api/v1/stats')
    })
    expect(mockGET).toHaveBeenCalledWith('/api/v1/mcp/servers', {
      params: { query: { limit: 5 } },
    })
    expect(mockGET).toHaveBeenCalledWith('/api/v1/agents', {
      params: { query: { limit: 5 } },
    })
  })

  it('renders stat tile counts from the API response', async () => {
    renderPage()
    expect(await screen.findByText('12')).toBeInTheDocument() // mcp_servers
    expect(screen.getByText('7')).toBeInTheDocument() // agents
    expect(screen.getByText('3')).toBeInTheDocument() // publishers
  })

  it('renders quick action links to create new entries', () => {
    renderPage()
    expect(screen.getByRole('link', { name: /new publisher/i })).toHaveAttribute(
      'href',
      '/admin/publishers/new',
    )
    expect(screen.getByRole('link', { name: /new mcp server/i })).toHaveAttribute(
      'href',
      '/admin/mcp/new',
    )
    expect(screen.getByRole('link', { name: /new agent/i })).toHaveAttribute(
      'href',
      '/admin/agents/new',
    )
  })

  it('renders recent MCP servers and recent agents with links to detail pages', async () => {
    renderPage()
    expect(await screen.findByText('Example MCP')).toBeInTheDocument()
    expect(screen.getByText('Example Agent')).toBeInTheDocument()
    const mcpLink = screen
      .getAllByRole('link')
      .find((a) => a.getAttribute('href') === '/admin/mcp/acme/example-mcp')
    const agentLink = screen
      .getAllByRole('link')
      .find((a) => a.getAttribute('href') === '/admin/agents/acme/example-agent')
    expect(mcpLink).toBeTruthy()
    expect(agentLink).toBeTruthy()
  })

  it('shows an error alert when stats fail to load', async () => {
    mockGET.mockImplementation((path: string) => {
      if (path === '/api/v1/stats') return Promise.reject(new Error('boom'))
      return Promise.resolve({ data: { items: [] } })
    })
    renderPage()
    expect(await screen.findByTestId('stats-error')).toBeInTheDocument()
  })
})
