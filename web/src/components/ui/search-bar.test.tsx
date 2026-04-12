import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'

// Mock navigate
const mockNavigate = vi.fn()
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual('react-router-dom')
  return { ...actual, useNavigate: () => mockNavigate }
})

// Mock API client
const mockGET = vi.fn()
vi.mock('@/lib/api-client', () => ({
  getPublicClient: () => ({ GET: mockGET }),
}))

import { SearchBar } from './search-bar'

function renderSearchBar() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter>
        <SearchBar />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

describe('SearchBar', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockGET.mockResolvedValue({ data: { items: [] } })
  })

  it('renders the search input', () => {
    renderSearchBar()
    expect(screen.getByRole('textbox', { name: /search registry/i })).toBeInTheDocument()
  })

  it('has placeholder text', () => {
    renderSearchBar()
    expect(screen.getByPlaceholderText(/search mcp servers and agents/i)).toBeInTheDocument()
  })

  it('navigates to /mcp?q=... on Enter', async () => {
    const user = userEvent.setup()
    renderSearchBar()
    const input = screen.getByRole('textbox')
    await user.type(input, 'weather')
    await user.keyboard('{Enter}')
    expect(mockNavigate).toHaveBeenCalledWith('/explore?q=weather')
  })

  it('does not navigate on Enter when query is empty', async () => {
    const user = userEvent.setup()
    renderSearchBar()
    await user.keyboard('{Enter}')
    expect(mockNavigate).not.toHaveBeenCalled()
  })

  it('shows dropdown results after debounce', async () => {
    mockGET.mockImplementation((path: string) => {
      if (path.includes('mcp')) {
        return Promise.resolve({
          data: { items: [{ id: '1', name: 'Weather MCP', namespace: 'acme', slug: 'weather' }] },
        })
      }
      return Promise.resolve({ data: { items: [] } })
    })

    const user = userEvent.setup()
    renderSearchBar()
    const input = screen.getByRole('textbox')
    await user.type(input, 'weather')

    await waitFor(() => {
      expect(screen.getByText('Weather MCP')).toBeInTheDocument()
    }, { timeout: 2000 })
  })

  it('closes dropdown on outside click', async () => {
    mockGET.mockImplementation((path: string) => {
      if (path.includes('mcp')) {
        return Promise.resolve({
          data: { items: [{ id: '1', name: 'Test Server', namespace: 'ns', slug: 'test' }] },
        })
      }
      return Promise.resolve({ data: { items: [] } })
    })

    const user = userEvent.setup()
    renderSearchBar()
    await user.type(screen.getByRole('textbox'), 'test')

    await waitFor(() => {
      expect(screen.getByText('Test Server')).toBeInTheDocument()
    }, { timeout: 2000 })

    // Click outside
    await user.click(document.body)
    await waitFor(() => {
      expect(screen.queryByText('Test Server')).not.toBeInTheDocument()
    })
  })

  it('navigates to detail page when clicking a result', async () => {
    mockGET.mockImplementation((path: string) => {
      if (path.includes('mcp')) {
        return Promise.resolve({
          data: { items: [{ id: '1', name: 'My Server', namespace: 'acme', slug: 'my-server' }] },
        })
      }
      return Promise.resolve({ data: { items: [] } })
    })

    const user = userEvent.setup()
    renderSearchBar()
    await user.type(screen.getByRole('textbox'), 'my')

    await waitFor(() => {
      expect(screen.getByText('My Server')).toBeInTheDocument()
    }, { timeout: 2000 })

    await user.click(screen.getByText('My Server'))
    expect(mockNavigate).toHaveBeenCalledWith('/mcp/acme/my-server')
  })
})
