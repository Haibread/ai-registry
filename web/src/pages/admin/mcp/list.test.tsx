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
    renderPage([
      '/admin/mcp?q=file&namespace=acme&status=published&visibility=public&cursor=c1',
    ])
    await waitFor(() => {
      expect(mockGET).toHaveBeenCalledWith('/api/v1/mcp/servers', {
        params: {
          query: {
            limit: 50,
            q: 'file',
            namespace: 'acme',
            cursor: 'c1',
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
    expect(screen.getByText('acme/fs')).toBeInTheDocument()
  })

  it('shows the bulk action bar when a row is selected', async () => {
    renderPage()
    await screen.findByText('Filesystem Server')
    fireEvent.click(screen.getByRole('checkbox', { name: /select filesystem server/i }))
    const toolbar = await screen.findByRole('toolbar', { name: /bulk actions/i })
    expect(toolbar).toBeInTheDocument()
    expect(screen.getByText(/1 selected/i)).toBeInTheDocument()
  })

  it('calls DELETE for each selected row when bulk-delete confirmed', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true)
    renderPage()
    await screen.findByText('Filesystem Server')
    fireEvent.click(screen.getByRole('checkbox', { name: /select filesystem server/i }))
    await screen.findByRole('toolbar', { name: /bulk actions/i })
    fireEvent.click(screen.getByRole('button', { name: /delete/i }))
    await waitFor(() => {
      expect(mockDELETE).toHaveBeenCalledWith(
        '/api/v1/mcp/servers/{namespace}/{slug}',
        { params: { path: { namespace: 'acme', slug: 'fs' } } },
      )
    })
    confirmSpy.mockRestore()
  })

  it('renders a Load more link when next_cursor is set', async () => {
    mockGET.mockResolvedValue({
      data: { items: sampleServers, next_cursor: 'next-c' },
    })
    renderPage(['/admin/mcp?q=file'])
    const loadMore = await screen.findByRole('link', { name: /load more/i })
    expect(loadMore.getAttribute('href')).toMatch(/\/admin\/mcp\?/)
    expect(loadMore.getAttribute('href')).toMatch(/cursor=next-c/)
    expect(loadMore.getAttribute('href')).toMatch(/q=file/)
  })
})
