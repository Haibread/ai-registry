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
    expect(screen.getByPlaceholderText(/search mcp servers/i)).toBeInTheDocument()
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
    const input = screen.getByPlaceholderText(/search mcp servers/i)
    await user.type(input, 'postgres')
    await user.click(screen.getByRole('button', { name: /^search$/i }))
    // The search should trigger new queries with q=postgres
    expect(mockGET).toHaveBeenCalled()
  })

  it('renders MCP Servers section when data exists', async () => {
    mockGET.mockResolvedValue({
      data: {
        items: [{ id: '1', name: 'Test MCP', namespace: 'ns', slug: 'test', description: 'A test MCP server' }],
        total_count: 1,
      },
    })

    renderExplore()
    expect(await screen.findByText('Test MCP')).toBeInTheDocument()
  })

  it('shows "No results found" when searching with no matches', async () => {
    renderExplore(['/explore?q=nonexistent'])
    expect(await screen.findByText(/no results found/i)).toBeInTheDocument()
  })

  it('sends the submitted search term to the listing', async () => {
    const user = userEvent.setup()
    renderExplore()
    await user.type(screen.getByPlaceholderText(/search mcp servers/i), '  postgres  ')
    await user.click(screen.getByRole('button', { name: /^search$/i }))

    expect(await screen.findByText(/no entries match "postgres"/i)).toBeInTheDocument()
    expect(lastQuery('/api/v1/mcp/servers')).toMatchObject({ q: 'postgres', limit: 20 })
  })

  it('applies the sort and clears filters', async () => {
    const user = userEvent.setup()
    renderExplore(['/explore?q=db'])
    await screen.findByText(/no results found/i)

    await user.selectOptions(screen.getByRole('combobox', { name: /sort order/i }), 'name_asc')
    await waitFor(() => expect(lastQuery('/api/v1/mcp/servers')).toMatchObject({ q: 'db', sort: 'name_asc' }))

    await user.click(screen.getByRole('button', { name: /clear filters/i }))
    expect(await screen.findByText(/nothing here yet/i)).toBeInTheDocument()
    expect(screen.getByPlaceholderText(/search mcp servers/i)).toHaveValue('')
    expect(screen.getByRole('combobox', { name: /sort order/i })).toHaveValue('')
  })
})
