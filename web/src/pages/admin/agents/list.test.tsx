import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor, fireEvent } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'

vi.mock('@/auth/AuthContext', () => ({
  useAuth: () => ({ accessToken: 'test-token' }),
}))

// The list scopes to the selected publisher; default to the All-publishers
// (null) scope so URL filters drive the query unchanged.
vi.mock('@/auth/PublisherContext', () => ({
  usePublisher: () => ({
    currentSlug: null,
    current: null,
    publishers: [],
    isServerAdmin: true,
    setCurrent: () => {},
    isLoading: false,
  }),
}))

const mockGET = vi.fn()
const mockPOST = vi.fn()
const mockDELETE = vi.fn()
vi.mock('@/lib/api-client', () => ({
  useAuthClient: () => ({ GET: mockGET, POST: mockPOST, DELETE: mockDELETE }),
}))

// Default the caller to a Server Admin so role-gated affordances render.
vi.mock('@/auth/useMe', () => ({
  usePermissions: () => ({
    isLoading: false,
    isAuthenticated: true,
    isServerAdmin: true,
    grants: [],
    rolesOn: () => new Set(),
    canEdit: () => true,
    canReview: () => true,
    canAdmin: () => true,
    isEditorAnywhere: true,
    isReviewerAnywhere: true,
  }),
  useMe: () => ({ data: { authenticated: true, is_server_admin: true, grants: [] }, isLoading: false }),
  satisfiesRole: () => true,
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
    // cursor is no longer read from the URL — it is the infinite-query page
    // param, so the first page sends cursor: undefined.
    renderPage([
      '/admin/agents?q=rev&namespace=acme&status=draft&visibility=private',
    ])
    await waitFor(() => {
      expect(mockGET).toHaveBeenCalledWith('/api/v1/agents', {
        params: {
          query: {
            limit: 50,
            mine: true,
            q: 'rev',
            namespace: 'acme',
            cursor: undefined,
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
    // Namespace/slug renders twice: a hidden-on-small dedicated cell and
    // an inline-on-small line under the agent name. Both end up in the
    // DOM because JSDOM doesn't honor responsive display classes.
    expect(screen.getAllByText('acme/reviewer').length).toBeGreaterThan(0)
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

  it('shows a Load more button and fetches the next page on click', async () => {
    mockGET.mockResolvedValue({
      data: { items: sampleAgents, next_cursor: 'next-c' },
    })
    renderPage(['/admin/agents?namespace=acme'])
    const loadMore = await screen.findByRole('button', { name: /load more/i })
    fireEvent.click(loadMore)
    await waitFor(() => {
      expect(mockGET).toHaveBeenCalledWith(
        '/api/v1/agents',
        expect.objectContaining({
          params: expect.objectContaining({
            query: expect.objectContaining({ cursor: 'next-c', namespace: 'acme' }),
          }),
        }),
      )
    })
  })
})
