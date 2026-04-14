import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor, fireEvent } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'

vi.mock('@/auth/AuthContext', () => ({
  useAuth: () => ({ accessToken: 'test-token' }),
}))

const mockGET = vi.fn()
const mockPOST = vi.fn()
const mockDELETE = vi.fn()
vi.mock('@/lib/api-client', () => ({
  useAuthClient: () => ({ GET: mockGET, POST: mockPOST, DELETE: mockDELETE }),
}))

import AdminAgentList from './list'

function renderPage(initialEntries: string[] = ['/admin/agents']) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={initialEntries}>
        <AdminAgentList />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

const sampleAgents = [
  {
    id: '01HAG1',
    name: 'Code Reviewer',
    namespace: 'acme',
    slug: 'reviewer',
    status: 'published',
    visibility: 'public',
    updated_at: '2026-04-10T10:00:00Z',
  },
  {
    id: '01HAG2',
    name: 'Bug Hunter',
    namespace: 'acme',
    slug: 'bug-hunter',
    status: 'draft',
    visibility: 'private',
    updated_at: '2026-04-11T10:00:00Z',
  },
  {
    id: '01HAG3',
    name: 'Ghost Agent',
    namespace: 'acme',
    slug: 'ghost',
    status: 'deleted',
    visibility: 'public',
    updated_at: '2026-04-09T10:00:00Z',
  },
]

describe('AdminAgentList', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockGET.mockResolvedValue({ data: { items: sampleAgents } })
    mockPOST.mockResolvedValue({})
    mockDELETE.mockResolvedValue({})
  })

  it('renders the heading and the New Agent link', async () => {
    renderPage()
    expect(screen.getByRole('heading', { name: /agents/i })).toBeInTheDocument()
    const newLink = screen.getByRole('link', { name: /new agent/i })
    expect(newLink).toHaveAttribute('href', '/admin/agents/new')
  })

  it('calls GET with params derived from the URL', async () => {
    renderPage([
      '/admin/agents?q=rev&namespace=acme&status=draft&visibility=private&cursor=c1',
    ])
    await waitFor(() => {
      expect(mockGET).toHaveBeenCalledWith('/api/v1/agents', {
        params: {
          query: {
            limit: 50,
            q: 'rev',
            namespace: 'acme',
            cursor: 'c1',
            status: 'draft',
            visibility: 'private',
          },
        },
      })
    })
  })

  it('renders agent rows and hides soft-deleted agents', async () => {
    renderPage()
    expect(await screen.findByText('Code Reviewer')).toBeInTheDocument()
    expect(screen.getByText('Bug Hunter')).toBeInTheDocument()
    expect(screen.queryByText('Ghost Agent')).not.toBeInTheDocument()
    expect(screen.getByText('acme/reviewer')).toBeInTheDocument()
  })

  it('shows the bulk action bar when a row is selected', async () => {
    renderPage()
    await screen.findByText('Code Reviewer')
    fireEvent.click(screen.getByRole('checkbox', { name: /select code reviewer/i }))
    const toolbar = await screen.findByRole('toolbar', { name: /bulk actions/i })
    expect(toolbar).toBeInTheDocument()
    expect(screen.getByText(/1 selected/i)).toBeInTheDocument()
  })

  it('calls DELETE for each selected row when bulk-delete confirmed', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true)
    renderPage()
    await screen.findByText('Code Reviewer')
    fireEvent.click(screen.getByRole('checkbox', { name: /select code reviewer/i }))
    await screen.findByRole('toolbar', { name: /bulk actions/i })
    fireEvent.click(screen.getByRole('button', { name: /delete/i }))
    await waitFor(() => {
      expect(mockDELETE).toHaveBeenCalledWith(
        '/api/v1/agents/{namespace}/{slug}',
        { params: { path: { namespace: 'acme', slug: 'reviewer' } } },
      )
    })
    confirmSpy.mockRestore()
  })

  it('renders a Load more link when next_cursor is set', async () => {
    mockGET.mockResolvedValue({
      data: { items: sampleAgents, next_cursor: 'next-c' },
    })
    renderPage(['/admin/agents?namespace=acme'])
    const loadMore = await screen.findByRole('link', { name: /load more/i })
    expect(loadMore.getAttribute('href')).toMatch(/\/admin\/agents\?/)
    expect(loadMore.getAttribute('href')).toMatch(/cursor=next-c/)
    expect(loadMore.getAttribute('href')).toMatch(/namespace=acme/)
  })
})
