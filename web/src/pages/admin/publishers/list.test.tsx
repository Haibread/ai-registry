import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
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

import AdminPublisherList from './list'

function renderPage(initialEntries: string[] = ['/admin/publishers']) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={initialEntries}>
        <AdminPublisherList />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

const samplePublishers = [
  {
    id: '01HPUB1',
    slug: 'acme',
    name: 'Acme Corp',
    contact: 'dev@acme.test',
    verified: true,
    created_at: '2026-04-01T10:00:00Z',
  },
  {
    id: '01HPUB2',
    slug: 'globex',
    name: 'Globex Inc',
    contact: null,
    verified: false,
    created_at: '2026-04-02T10:00:00Z',
  },
]

describe('AdminPublisherList', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockGET.mockResolvedValue({ data: { items: samplePublishers } })
  })

  it('renders the page heading and New Publisher link', async () => {
    renderPage()
    expect(screen.getByRole('heading', { name: /publishers/i })).toBeInTheDocument()
    const newLink = screen.getByRole('link', { name: /new publisher/i })
    expect(newLink).toBeInTheDocument()
    expect(newLink).toHaveAttribute('href', '/admin/publishers/new')
  })

  it('fetches publishers with the expected limit param', async () => {
    renderPage()
    await waitFor(() => {
      expect(mockGET).toHaveBeenCalledWith('/api/v1/publishers', {
        params: { query: { limit: 100 } },
      })
    })
  })

  it('renders publisher rows from the API response', async () => {
    renderPage()
    expect(await screen.findByText('Acme Corp')).toBeInTheDocument()
    expect(screen.getByText('Globex Inc')).toBeInTheDocument()
    expect(screen.getByText('acme')).toBeInTheDocument()
    expect(screen.getByText('globex')).toBeInTheDocument()
  })

  it('renders verified / unverified indicators', async () => {
    renderPage()
    await screen.findByText('Acme Corp')
    expect(screen.getByLabelText(/^verified$/i)).toBeInTheDocument()
    expect(screen.getByLabelText(/^unverified$/i)).toBeInTheDocument()
  })

  it('renders a Manage link for each publisher', async () => {
    renderPage()
    await screen.findByText('Acme Corp')
    const manageLinks = screen.getAllByRole('link', { name: /manage/i })
    expect(manageLinks).toHaveLength(2)
    expect(manageLinks[0]).toHaveAttribute('href', '/admin/publishers/acme')
    expect(manageLinks[1]).toHaveAttribute('href', '/admin/publishers/globex')
  })

  it('shows the empty state when there are no publishers', async () => {
    mockGET.mockResolvedValueOnce({ data: { items: [] } })
    renderPage()
    expect(await screen.findByText(/no publishers yet/i)).toBeInTheDocument()
    expect(
      screen.getByRole('link', { name: /create your first publisher/i }),
    ).toBeInTheDocument()
  })
})
