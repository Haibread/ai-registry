import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor, fireEvent } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'

vi.mock('@/auth/AuthContext', () => ({
  useAuth: () => ({ accessToken: 'test-token' }),
}))

const mockGET = vi.fn()
const mockPATCH = vi.fn()
vi.mock('@/lib/api-client', () => ({
  useAuthClient: () => ({ GET: mockGET, PATCH: mockPATCH }),
}))

import AdminReports from './reports'

function renderPage() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter>
        <AdminReports />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

const sampleReport = {
  id: '01H',
  resource_type: 'mcp_server',
  resource_id: '01HMCP',
  issue_type: 'broken',
  description: 'Package fails to install on node 22.',
  status: 'pending',
  created_at: '2026-04-10T10:00:00Z',
  reporter_ip: '1.2.3.4',
  reviewed_by: '',
  reviewed_at: null,
}

describe('AdminReports', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockGET.mockResolvedValue({ data: { items: [sampleReport] } })
    mockPATCH.mockResolvedValue({})
  })

  it('renders the page heading', async () => {
    renderPage()
    expect(screen.getByRole('heading', { name: /reports/i })).toBeInTheDocument()
  })

  it('fetches pending reports on mount', async () => {
    renderPage()
    await waitFor(() => {
      expect(mockGET).toHaveBeenCalledWith('/api/v1/reports', {
        params: { query: { status: 'pending' } },
      })
    })
  })

  it('renders report rows from the API', async () => {
    renderPage()
    expect(await screen.findByText(/package fails to install/i)).toBeInTheDocument()
    expect(screen.getByText('01HMCP')).toBeInTheDocument()
  })

  it('switches filter to reviewed when tab clicked', async () => {
    renderPage()
    fireEvent.click(screen.getByRole('button', { name: /reviewed/i }))
    await waitFor(() => {
      expect(mockGET).toHaveBeenCalledWith('/api/v1/reports', {
        params: { query: { status: 'reviewed' } },
      })
    })
  })

  it('calls PATCH when marking a report reviewed', async () => {
    renderPage()
    await screen.findByText(/package fails to install/i)
    fireEvent.click(screen.getByRole('button', { name: /mark reviewed/i }))
    await waitFor(() => {
      expect(mockPATCH).toHaveBeenCalledWith('/api/v1/reports/{id}', {
        params: { path: { id: '01H' } },
        body: { status: 'reviewed' },
      })
    })
  })

  it('calls PATCH when dismissing a report', async () => {
    renderPage()
    await screen.findByText(/package fails to install/i)
    fireEvent.click(screen.getByRole('button', { name: /^dismiss$/i }))
    await waitFor(() => {
      expect(mockPATCH).toHaveBeenCalledWith('/api/v1/reports/{id}', {
        params: { path: { id: '01H' } },
        body: { status: 'dismissed' },
      })
    })
  })

  it('shows an empty state when there are no reports', async () => {
    mockGET.mockResolvedValueOnce({ data: { items: [] } })
    renderPage()
    expect(await screen.findByText(/no pending reports/i)).toBeInTheDocument()
  })
})
