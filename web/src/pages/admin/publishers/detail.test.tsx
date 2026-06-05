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
      if (path === '/api/v1/mcp/servers') return Promise.resolve({ data: { items: sampleMcp, total_count: 3 } })
      if (path === '/api/v1/agents') return Promise.resolve({ data: { items: sampleAgents, total_count: 2 } })
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

  it('reveals a confirmation panel with the cascade resource counts', async () => {
    renderPage()
    await screen.findByRole('heading', { name: 'Acme Corp' })
    fireEvent.click(screen.getByRole('button', { name: /^delete$/i }))

    // Uses the server-reported totals (3 / 2), not the page-capped item lists.
    expect(await screen.findByText(/3 MCP servers/i)).toBeInTheDocument()
    expect(screen.getByText(/2 agents/i)).toBeInTheDocument()
  })

  it('gates DELETE behind typing the exact publisher name', async () => {
    renderPage()
    await screen.findByRole('heading', { name: 'Acme Corp' })
    fireEvent.click(screen.getByRole('button', { name: /^delete$/i }))

    const submit = await screen.findByRole('button', { name: /delete publisher/i })
    expect(submit).toBeDisabled()

    const input = screen.getByLabelText(/to confirm/i)
    fireEvent.change(input, { target: { value: 'Acme' } })
    expect(submit).toBeDisabled()

    fireEvent.change(input, { target: { value: 'Acme Corp' } })
    expect(submit).toBeEnabled()

    fireEvent.click(submit)
    await waitFor(() => {
      expect(mockDELETE).toHaveBeenCalledWith('/api/v1/publishers/{slug}', {
        params: { path: { slug: 'acme' } },
      })
    })
  })

  it('does not call DELETE while the typed name does not match', async () => {
    renderPage()
    await screen.findByRole('heading', { name: 'Acme Corp' })
    fireEvent.click(screen.getByRole('button', { name: /^delete$/i }))

    const input = await screen.findByLabelText(/to confirm/i)
    fireEvent.change(input, { target: { value: 'wrong name' } })
    fireEvent.submit(input.closest('form') as HTMLFormElement)

    expect(mockDELETE).not.toHaveBeenCalled()
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
