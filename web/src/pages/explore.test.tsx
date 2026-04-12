import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, within } from '@testing-library/react'
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
})
