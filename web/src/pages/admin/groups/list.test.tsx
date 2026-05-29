import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'

vi.mock('@/auth/AuthContext', () => ({
  useAuth: () => ({ accessToken: 'test-token' }),
}))

const mockGET = vi.fn()
vi.mock('@/lib/api-client', () => ({
  useAuthClient: () => ({ GET: mockGET }),
}))

import AdminGroupList from './list'

function renderPage() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={['/admin/groups']}>
        <AdminGroupList />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

const sampleGroups = [
  { id: '01HG1', slug: 'platform', name: 'Platform', description: 'infra', created_at: '2026-04-01T10:00:00Z', updated_at: '2026-04-01T10:00:00Z' },
  { id: '01HG2', slug: 'reviewers', name: 'Reviewers', created_at: '2026-04-02T10:00:00Z', updated_at: '2026-04-02T10:00:00Z' },
]

describe('AdminGroupList', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockGET.mockResolvedValue({ data: { items: sampleGroups } })
  })

  it('renders heading and New Group link', () => {
    renderPage()
    expect(screen.getByRole('heading', { name: /groups/i })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /new group/i })).toHaveAttribute('href', '/admin/groups/new')
  })

  it('fetches groups with the expected limit', async () => {
    renderPage()
    await waitFor(() => {
      expect(mockGET).toHaveBeenCalledWith('/api/v1/groups', { params: { query: { limit: 100 } } })
    })
  })

  it('renders group rows with Manage links', async () => {
    renderPage()
    expect(await screen.findByText('Platform')).toBeInTheDocument()
    expect(screen.getByText('reviewers')).toBeInTheDocument()
    const manage = screen.getAllByRole('link', { name: /manage/i })
    expect(manage[0]).toHaveAttribute('href', '/admin/groups/platform')
  })

  it('shows the empty state', async () => {
    mockGET.mockResolvedValueOnce({ data: { items: [] } })
    renderPage()
    expect(await screen.findByText(/no groups yet/i)).toBeInTheDocument()
  })
})
