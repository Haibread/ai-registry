import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'

// Mock auth + theme (Header uses both)
vi.mock('@/auth/AuthContext', () => ({
  useAuth: () => ({
    accessToken: null,
    login: vi.fn(),
    logout: vi.fn(),
    loginError: null,
  }),
}))
vi.mock('@/components/providers', () => ({
  useTheme: () => ({ theme: 'light', setTheme: vi.fn() }),
}))

// Mock API client
const mockGET = vi.fn()
vi.mock('@/lib/api-client', () => ({
  getPublicClient: () => ({ GET: mockGET }),
}))

import ExplorePage from './explore'

function rows(prefix: string, n: number) {
  return Array.from({ length: n }, (_, i) => ({
    id: `${prefix}-${i}`,
    name: `${prefix} ${i}`,
    namespace: 'ns',
    slug: `${prefix}-${i}`,
    description: 'desc',
  }))
}

function primeBoth(mcpCount: number, agentCount: number) {
  mockGET.mockImplementation((path: string) =>
    Promise.resolve({
      data: path.includes('mcp')
        ? { items: rows('Server', mcpCount), total_count: mcpCount }
        : { items: rows('Agent', agentCount), total_count: agentCount },
    }),
  )
}

function lastQuery(path: string): Record<string, unknown> | undefined {
  const calls = mockGET.mock.calls.filter(([p]) => p === path)
  return (calls[calls.length - 1]?.[1] as { params: { query: Record<string, unknown> } } | undefined)?.params.query
}

function renderExplore(initialEntries = ['/explore']) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={initialEntries}>
        <ExplorePage />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

describe('ExplorePage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockGET.mockResolvedValue({ data: { items: [], total_count: 0 } })
  })

  it('renders the heading', () => {
    renderExplore()
    expect(screen.getByRole('heading', { name: /explore/i })).toBeInTheDocument()
  })

  it('renders the search input', () => {
    renderExplore()
    expect(screen.getByPlaceholderText(/search everything/i)).toBeInTheDocument()
  })

  it('renders type tabs: All, MCP Servers, Agents', () => {
    renderExplore()
    expect(screen.getByRole('button', { name: /^all$/i })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /mcp servers/i })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /agents/i })).toBeInTheDocument()
  })

  it('renders sort select', () => {
    renderExplore()
    expect(screen.getByRole('combobox', { name: /sort order/i })).toBeInTheDocument()
  })

  it('shows empty state when no results', async () => {
    renderExplore()
    // Wait for queries to settle
    expect(await screen.findByText(/nothing here yet/i)).toBeInTheDocument()
  })

  it('searches on form submit', async () => {
    const user = userEvent.setup()
    renderExplore()
    const input = screen.getByPlaceholderText(/search everything/i)
    await user.type(input, 'postgres')
    await user.click(screen.getByRole('button', { name: /^search$/i }))
    // The search should trigger new queries with q=postgres
    expect(mockGET).toHaveBeenCalled()
  })

  it('renders MCP Servers section when data exists', async () => {
    mockGET.mockImplementation((path: string) => {
      if (path.includes('mcp')) {
        return Promise.resolve({
          data: {
            items: [{ id: '1', name: 'Test MCP', namespace: 'ns', slug: 'test', description: 'A test MCP server' }],
            total_count: 1,
          },
        })
      }
      return Promise.resolve({ data: { items: [], total_count: 0 } })
    })

    renderExplore()
    expect(await screen.findByText('Test MCP')).toBeInTheDocument()
  })

  it('renders Agents section when data exists', async () => {
    mockGET.mockImplementation((path: string) => {
      if (path.includes('agents')) {
        return Promise.resolve({
          data: {
            items: [{ id: '2', name: 'Test Agent', namespace: 'ns', slug: 'test-agent', description: 'A test agent' }],
            total_count: 1,
          },
        })
      }
      return Promise.resolve({ data: { items: [], total_count: 0 } })
    })

    renderExplore()
    expect(await screen.findByText('Test Agent')).toBeInTheDocument()
  })

  it('shows "No results found" when searching with no matches', async () => {
    renderExplore(['/explore?q=nonexistent'])
    expect(await screen.findByText(/no results found/i)).toBeInTheDocument()
  })

  it('sends the submitted search term to both listings', async () => {
    const user = userEvent.setup()
    renderExplore()
    await user.type(screen.getByPlaceholderText(/search everything/i), '  postgres  ')
    await user.click(screen.getByRole('button', { name: /^search$/i }))

    expect(await screen.findByText(/no entries match "postgres"/i)).toBeInTheDocument()
    expect(lastQuery('/api/v1/mcp/servers')).toMatchObject({ q: 'postgres', limit: 6 })
    expect(lastQuery('/api/v1/agents')).toMatchObject({ q: 'postgres', limit: 6 })
  })

  it('offers "View all" links carrying the query once a section is full', async () => {
    primeBoth(6, 6)
    renderExplore(['/explore?q=db'])

    await screen.findByText('Server 5')
    const links = screen.getAllByRole('link', { name: /view all/i }).map((l) => l.getAttribute('href'))
    expect(links).toEqual(['/mcp?q=db', '/agents?q=db'])
  })

  it('shows only the selected type and asks for a larger page', async () => {
    const user = userEvent.setup()
    primeBoth(2, 2)
    renderExplore(['/explore?type=agents'])

    expect(await screen.findByText('Agent 0')).toBeInTheDocument()
    expect(screen.queryByText('Server 0')).not.toBeInTheDocument()
    expect(lastQuery('/api/v1/mcp/servers')).toBeUndefined()
    expect(lastQuery('/api/v1/agents')).toMatchObject({ limit: 20 })
    expect(screen.queryByRole('link', { name: /view all/i })).not.toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: /mcp servers/i }))
    expect(await screen.findByText('Server 0')).toBeInTheDocument()
    expect(screen.queryByText('Agent 0')).not.toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: /^all$/i }))
    expect(await screen.findByText('Agent 0')).toBeInTheDocument()
    expect(screen.getByText('Server 0')).toBeInTheDocument()
  })

  it('refetches a full page when switching from All to a single-type tab', async () => {
    const user = userEvent.setup()
    mockGET.mockImplementation((path: string, init: { params: { query: { limit: number } } }) => {
      const { limit } = init.params.query
      return Promise.resolve({
        data: path.includes('mcp')
          ? { items: rows('Server', limit), total_count: 40 }
          : { items: rows('Agent', limit), total_count: 40 },
      })
    })
    renderExplore()

    expect(await screen.findByText('Server 5')).toBeInTheDocument()
    expect(screen.queryByText('Server 6')).not.toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: /mcp servers/i }))
    expect(await screen.findByText('Server 19')).toBeInTheDocument()
    expect(lastQuery('/api/v1/mcp/servers')).toMatchObject({ limit: 20 })
  })

  it('applies the sort and clears filters while keeping the type tab', async () => {
    const user = userEvent.setup()
    renderExplore(['/explore?type=mcp&q=db'])
    await screen.findByText(/no results found/i)

    await user.selectOptions(screen.getByRole('combobox', { name: /sort order/i }), 'name_asc')
    await waitFor(() => expect(lastQuery('/api/v1/mcp/servers')).toMatchObject({ q: 'db', sort: 'name_asc' }))

    await user.click(screen.getByRole('button', { name: /clear filters/i }))
    expect(await screen.findByText(/nothing here yet/i)).toBeInTheDocument()
    expect(screen.getByPlaceholderText(/search everything/i)).toHaveValue('')
    expect(screen.getByRole('combobox', { name: /sort order/i })).toHaveValue('')
    expect(lastQuery('/api/v1/agents')).toBeUndefined()
  })
})
