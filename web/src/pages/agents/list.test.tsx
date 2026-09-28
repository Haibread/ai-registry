import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import type { components } from '@/lib/schema'

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

import AgentListPage from './list'

type Agent = components['schemas']['Agent']
type AgentList = { items: Agent[]; total_count?: number; next_cursor?: string }

function makeAgent(slug: string, name: string): Agent {
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
  } as Agent
}

function primeGET(list: AgentList) {
  mockGET.mockImplementation((path: string) => {
    if (path === '/api/v1/tags') {
      return Promise.resolve({ data: { items: [{ slug: 'db', name: 'Databases', active: true }] } })
    }
    return Promise.resolve({ data: list })
  })
}

function renderAt(url: string) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={[url]}>
        <Routes>
          <Route path="/agents" element={<AgentListPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

function agentListQuery(): Record<string, unknown> | undefined {
  const call = mockGET.mock.calls.find(([path]) => path === '/api/v1/agents')
  return (call?.[1] as { params: { query: Record<string, unknown> } } | undefined)?.params.query
}

describe('AgentListPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders a card per agent and the paged count', async () => {
    primeGET({ items: [makeAgent('files', 'Review Bot'), makeAgent('web', 'Triage Bot')], total_count: 5 })
    renderAt('/agents')

    expect(await screen.findByText('Review Bot')).toBeInTheDocument()
    expect(screen.getByText('Triage Bot')).toBeInTheDocument()
    expect(screen.getByText('Showing 2 of 5 agents')).toBeInTheDocument()
  })

  it('uses the singular and omits the total when everything is shown', async () => {
    primeGET({ items: [makeAgent('files', 'Review Bot')], total_count: 1 })
    renderAt('/agents')

    expect(await screen.findByText('Showing 1 agent')).toBeInTheDocument()
  })

  it('forwards the URL filters to the API', async () => {
    primeGET({ items: [] })
    renderAt('/agents?q=files&namespace=acme&status=deprecated&tag=db&sort=name_asc&cursor=c1')

    await screen.findByText('No agents match your filters.')
    expect(agentListQuery()).toEqual({
      q: 'files',
      cursor: 'c1',
      limit: 20,
      namespace: 'acme',
      status: 'deprecated',
      tag: 'db',
      sort: 'name_asc',
    })
  })

  it('shows the empty registry state without filters', async () => {
    primeGET({ items: [] })
    renderAt('/agents')

    expect(await screen.findByText('No public agents yet.')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /clear filters/i })).not.toBeInTheDocument()
  })

  it('clears every filter from the empty state', async () => {
    const user = userEvent.setup()
    primeGET({ items: [] })
    renderAt('/agents?q=nothing-matches')

    expect(await screen.findByText('No agents match your filters.')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: /clear filters/i }))
    expect(await screen.findByText('No public agents yet.')).toBeInTheDocument()
  })

  it('links "Load more" to the next cursor, keeping the current filters', async () => {
    primeGET({ items: [makeAgent('files', 'Review Bot')], total_count: 40, next_cursor: 'next-1' })
    renderAt('/agents?q=files&cursor=prev')

    const link = await screen.findByRole('link', { name: /load more/i })
    expect(link).toHaveAttribute('href', '/agents?q=files&cursor=next-1')
  })

  it('hides "Load more" on the last page', async () => {
    primeGET({ items: [makeAgent('files', 'Review Bot')] })
    renderAt('/agents')

    await screen.findByText('Review Bot')
    expect(screen.queryByRole('link', { name: /load more/i })).not.toBeInTheDocument()
  })
})
