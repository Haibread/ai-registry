import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor, fireEvent, within } from '@testing-library/react'
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

// Default the caller to a Server Admin so role-gated affordances (New, bulk
// delete/visibility) render; per-role gating is covered in useMe.test.ts.
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

import AdminMCPList from './list'

function renderPage(initialEntries: string[] = ['/admin/mcp']) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={initialEntries}>
        <AdminMCPList />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

const sampleServers = [
  {
    id: '01HSRV1',
    name: 'Filesystem Server',
    namespace: 'acme',
    slug: 'fs',
    status: 'published',
    visibility: 'public',
    updated_at: '2026-04-10T10:00:00Z',
  },
  {
    id: '01HSRV2',
    name: 'Memory Server',
    namespace: 'acme',
    slug: 'memory',
    status: 'draft',
    visibility: 'private',
    updated_at: '2026-04-11T10:00:00Z',
  },
]


// JSDOM doesn't implement HTMLDialogElement's modal methods; stub them so the
// bulk-action ConfirmDialog opens and closes.
beforeEach(() => {
  if (!HTMLDialogElement.prototype.showModal) {
    HTMLDialogElement.prototype.showModal = function () {
      this.setAttribute('open', '')
    }
  }
  if (!HTMLDialogElement.prototype.close) {
    HTMLDialogElement.prototype.close = function () {
      this.removeAttribute('open')
      this.dispatchEvent(new Event('close'))
    }
  }
})

describe('AdminMCPList', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockGET.mockResolvedValue({ data: { items: sampleServers } })
    mockPOST.mockResolvedValue({})
    mockDELETE.mockResolvedValue({})
  })

  it('renders the heading and the New Server link', async () => {
    renderPage()
    expect(screen.getByRole('heading', { name: /mcp servers/i })).toBeInTheDocument()
    const newLink = screen.getByRole('link', { name: /new server/i })
    expect(newLink).toHaveAttribute('href', '/admin/mcp/new')
  })

  it('calls GET with params derived from the URL', async () => {
    // cursor is no longer read from the URL — it is the infinite-query page
    // param, so the first page sends cursor: undefined.
    renderPage([
      '/admin/mcp?q=file&namespace=acme&status=published&visibility=public',
    ])
    await waitFor(() => {
      expect(mockGET).toHaveBeenCalledWith('/api/v1/mcp/servers', {
        params: {
          query: {
            limit: 50,
            mine: true,
            q: 'file',
            namespace: 'acme',
            cursor: undefined,
            status: 'published',
            visibility: 'public',
          },
        },
      })
    })
  })

  it('renders server rows from the API response', async () => {
    renderPage()
    expect(await screen.findByText('Filesystem Server')).toBeInTheDocument()
    expect(screen.getByText('Memory Server')).toBeInTheDocument()
    // The namespace/slug renders both as a hidden-on-small dedicated cell
    // and inline under the name on small screens, so it appears twice in
    // the DOM (one of the two is hidden via Tailwind's responsive classes).
    expect(screen.getAllByText('acme/fs').length).toBeGreaterThan(0)
  })

  it('shows the bulk action bar when a row is selected', async () => {
    renderPage()
    await screen.findByText('Filesystem Server')
    fireEvent.click(screen.getByRole('checkbox', { name: /select filesystem server/i }))
    const toolbar = await screen.findByRole('toolbar', { name: /bulk actions/i })
    expect(toolbar).toBeInTheDocument()
    expect(screen.getByText(/1 selected/i)).toBeInTheDocument()
  })

  it('calls DELETE for each selected row when bulk-delete is confirmed in the dialog', async () => {
    renderPage()
    await screen.findByText('Filesystem Server')
    fireEvent.click(screen.getByRole('checkbox', { name: /select filesystem server/i }))
    await screen.findByRole('toolbar', { name: /bulk actions/i })
    fireEvent.click(screen.getByRole('button', { name: /delete/i }))
    // The bulk bar click opens the shared confirmation; nothing fires yet.
    expect(mockDELETE).not.toHaveBeenCalled()
    expect(screen.getByRole('heading', { name: /delete 1 server\?/i })).toBeInTheDocument()
    const dialog = document.querySelector('dialog')!
    fireEvent.click(within(dialog).getByRole('button', { name: /^delete$/i }))
    await waitFor(() => {
      expect(mockDELETE).toHaveBeenCalledWith(
        '/api/v1/mcp/servers/{namespace}/{slug}',
        { params: { path: { namespace: 'acme', slug: 'fs' } } },
      )
    })
  })

  it('shows a Load more button and fetches the next page on click', async () => {
    mockGET.mockResolvedValue({
      data: { items: sampleServers, next_cursor: 'next-c' },
    })
    renderPage(['/admin/mcp?q=file'])
    const loadMore = await screen.findByRole('button', { name: /load more/i })
    fireEvent.click(loadMore)
    // The next page request carries the cursor returned by the first page.
    await waitFor(() => {
      expect(mockGET).toHaveBeenCalledWith(
        '/api/v1/mcp/servers',
        expect.objectContaining({
          params: expect.objectContaining({
            query: expect.objectContaining({ cursor: 'next-c', q: 'file' }),
          }),
        }),
      )
    })
  })
})
