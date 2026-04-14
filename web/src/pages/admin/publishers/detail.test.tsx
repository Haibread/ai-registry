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

import AdminPublisherDetail from './detail'

function renderPage() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={['/admin/publishers/acme']}>
        <Routes>
          <Route path="/admin/publishers/:slug" element={<AdminPublisherDetail />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

const samplePublisher = {
  id: '01HPUB1',
  slug: 'acme',
  name: 'Acme Corp',
  contact: 'dev@acme.test',
  verified: true,
  created_at: '2026-04-01T10:00:00Z',
  updated_at: '2026-04-02T10:00:00Z',
}

const sampleMcp = [
  {
    id: '01HMCP1',
    name: 'Acme MCP',
    namespace: 'acme',
    slug: 'acme-mcp',
    status: 'published',
    updated_at: '2026-04-03T10:00:00Z',
  },
]

const sampleAgents = [
  {
    id: '01HAGT1',
    name: 'Acme Agent',
    namespace: 'acme',
    slug: 'acme-agent',
    status: 'draft',
    updated_at: '2026-04-04T10:00:00Z',
  },
]

describe('AdminPublisherDetail', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockGET.mockImplementation((path: string) => {
      if (path === '/api/v1/publishers/{slug}') return Promise.resolve({ data: samplePublisher })
      if (path === '/api/v1/mcp/servers') return Promise.resolve({ data: { items: sampleMcp } })
      if (path === '/api/v1/agents') return Promise.resolve({ data: { items: sampleAgents } })
      return Promise.resolve({ data: {} })
    })
    mockPATCH.mockResolvedValue({})
    mockDELETE.mockResolvedValue({})
  })

  it('fetches the publisher and its child MCP servers / agents on mount', async () => {
    renderPage()
    await waitFor(() => {
      expect(mockGET).toHaveBeenCalledWith('/api/v1/publishers/{slug}', {
        params: { path: { slug: 'acme' } },
      })
    })
    expect(mockGET).toHaveBeenCalledWith('/api/v1/mcp/servers', {
      params: { query: { namespace: 'acme', limit: 50 } },
    })
    expect(mockGET).toHaveBeenCalledWith('/api/v1/agents', {
      params: { query: { namespace: 'acme', limit: 50 } },
    })
  })

  it('renders the publisher name, slug, and contact', async () => {
    renderPage()
    expect(await screen.findByRole('heading', { name: 'Acme Corp' })).toBeInTheDocument()
    expect(screen.getByText('dev@acme.test')).toBeInTheDocument()
  })

  it('renders the child MCP server and agent rows', async () => {
    renderPage()
    expect(await screen.findByText('Acme MCP')).toBeInTheDocument()
    expect(screen.getByText('Acme Agent')).toBeInTheDocument()
  })

  it('opens the edit form and submits a PATCH on save', async () => {
    renderPage()
    await screen.findByRole('heading', { name: 'Acme Corp' })
    fireEvent.click(screen.getByRole('button', { name: /^edit$/i }))
    fireEvent.click(screen.getByRole('button', { name: /save changes/i }))
    await waitFor(() => {
      expect(mockPATCH).toHaveBeenCalledWith('/api/v1/publishers/{slug}', {
        params: { path: { slug: 'acme' } },
        body: { name: 'Acme Corp', contact: 'dev@acme.test' },
      })
    })
  })

  it('calls DELETE when confirming the delete dialog', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true)
    renderPage()
    await screen.findByRole('heading', { name: 'Acme Corp' })
    fireEvent.click(screen.getByRole('button', { name: /^delete$/i }))
    await waitFor(() => {
      expect(mockDELETE).toHaveBeenCalledWith('/api/v1/publishers/{slug}', {
        params: { path: { slug: 'acme' } },
      })
    })
    confirmSpy.mockRestore()
  })

  it('shows a not-found state when the publisher query errors', async () => {
    mockGET.mockImplementation((path: string) => {
      if (path === '/api/v1/publishers/{slug}') return Promise.reject(new Error('nope'))
      return Promise.resolve({ data: { items: [] } })
    })
    renderPage()
    expect(await screen.findByText(/not found/i)).toBeInTheDocument()
  })
})
