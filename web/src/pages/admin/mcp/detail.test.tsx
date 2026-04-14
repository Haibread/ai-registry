import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor, fireEvent } from '@testing-library/react'
import { MemoryRouter, Routes, Route } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'

vi.mock('@/auth/AuthContext', () => ({
  useAuth: () => ({ accessToken: 'test-token' }),
}))

const mockGET = vi.fn()
const mockPOST = vi.fn()
const mockPATCH = vi.fn()
const mockDELETE = vi.fn()
vi.mock('@/lib/api-client', () => ({
  useAuthClient: () => ({ GET: mockGET, POST: mockPOST, PATCH: mockPATCH, DELETE: mockDELETE }),
}))

import AdminMCPDetail from './detail'

function renderPage() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={['/admin/mcp/acme/example-mcp']}>
        <Routes>
          <Route path="/admin/mcp/:ns/:slug" element={<AdminMCPDetail />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

const sampleServer = {
  id: '01HMCP1',
  namespace: 'acme',
  slug: 'example-mcp',
  name: 'Example MCP',
  description: 'An example MCP server',
  status: 'published',
  visibility: 'public',
  license: 'MIT',
  homepage_url: 'https://example.test',
  repo_url: 'https://github.com/acme/example',
  created_at: '2026-04-01T10:00:00Z',
  updated_at: '2026-04-02T10:00:00Z',
  latest_version: {
    version: '1.2.3',
    runtime: 'node',
    protocol_version: '2025-06-18',
    published_at: '2026-04-02T10:00:00Z',
    packages: [],
  },
}

describe('AdminMCPDetail', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockGET.mockResolvedValue({ data: sampleServer })
    mockPOST.mockResolvedValue({})
    mockPATCH.mockResolvedValue({})
    mockDELETE.mockResolvedValue({})
  })

  it('fetches the server detail on mount', async () => {
    renderPage()
    await waitFor(() => {
      expect(mockGET).toHaveBeenCalledWith('/api/v1/mcp/servers/{namespace}/{slug}', {
        params: { path: { namespace: 'acme', slug: 'example-mcp' } },
      })
    })
  })

  it('renders the heading and key metadata fields', async () => {
    renderPage()
    expect(await screen.findByRole('heading', { name: 'Example MCP' })).toBeInTheDocument()
    expect(screen.getByText('An example MCP server')).toBeInTheDocument()
    expect(screen.getByText('v1.2.3')).toBeInTheDocument()
    expect(screen.getByText('MIT')).toBeInTheDocument()
  })

  it('toggles visibility via POST when the make-private button is clicked', async () => {
    renderPage()
    await screen.findByRole('heading', { name: 'Example MCP' })
    fireEvent.click(screen.getByRole('button', { name: /make private/i }))
    await waitFor(() => {
      expect(mockPOST).toHaveBeenCalledWith(
        '/api/v1/mcp/servers/{namespace}/{slug}/visibility',
        {
          params: { path: { namespace: 'acme', slug: 'example-mcp' } },
          body: { visibility: 'private' },
        },
      )
    })
  })

  it('deprecates the server when the confirm dialog is accepted', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true)
    renderPage()
    await screen.findByRole('heading', { name: 'Example MCP' })
    fireEvent.click(screen.getByRole('button', { name: /^deprecate$/i }))
    await waitFor(() => {
      expect(mockPOST).toHaveBeenCalledWith(
        '/api/v1/mcp/servers/{namespace}/{slug}/deprecate',
        { params: { path: { namespace: 'acme', slug: 'example-mcp' } } },
      )
    })
    confirmSpy.mockRestore()
  })

  it('submits a PATCH when the edit form is saved', async () => {
    renderPage()
    await screen.findByRole('heading', { name: 'Example MCP' })
    fireEvent.click(screen.getByRole('button', { name: /^edit$/i }))
    fireEvent.click(screen.getByRole('button', { name: /save changes/i }))
    await waitFor(() => {
      expect(mockPATCH).toHaveBeenCalledWith('/api/v1/mcp/servers/{namespace}/{slug}', {
        params: { path: { namespace: 'acme', slug: 'example-mcp' } },
        body: {
          name: 'Example MCP',
          description: 'An example MCP server',
          homepage_url: 'https://example.test',
          repo_url: 'https://github.com/acme/example',
          license: 'MIT',
        },
      })
    })
  })

  it('shows a not-found state when the query errors', async () => {
    mockGET.mockRejectedValueOnce(new Error('nope'))
    renderPage()
    expect(await screen.findByText(/not found/i)).toBeInTheDocument()
  })
})
