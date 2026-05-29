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

import AdminUserList from './list'

function renderPage() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={['/admin/users']}>
        <AdminUserList />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

const sampleUsers = [
  { id: '01HU1', email: 'admin@x.test', display_name: 'Admin', has_password: true, is_server_admin: true, disabled: false, created_at: '2026-04-01T10:00:00Z', updated_at: '2026-04-01T10:00:00Z' },
  { id: '01HU2', email: 'gone@x.test', has_password: false, is_server_admin: false, disabled: true, created_at: '2026-04-02T10:00:00Z', updated_at: '2026-04-02T10:00:00Z' },
]

describe('AdminUserList', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockGET.mockResolvedValue({ data: { items: sampleUsers } })
  })

  it('renders heading and New User link', () => {
    renderPage()
    expect(screen.getByRole('heading', { name: /users/i })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /new user/i })).toHaveAttribute('href', '/admin/users/new')
  })

  it('fetches users with the expected limit', async () => {
    renderPage()
    await waitFor(() => {
      expect(mockGET).toHaveBeenCalledWith('/api/v1/users', { params: { query: { limit: 100 } } })
    })
  })

  it('renders rows with server-admin and disabled indicators', async () => {
    renderPage()
    expect(await screen.findByText('admin@x.test')).toBeInTheDocument()
    expect(screen.getByText(/server admin/i)).toBeInTheDocument()
    expect(screen.getByText(/disabled/i)).toBeInTheDocument()
    const manage = screen.getAllByRole('link', { name: /manage/i })
    expect(manage[0]).toHaveAttribute('href', '/admin/users/01HU1')
  })

  it('shows the empty state', async () => {
    mockGET.mockResolvedValueOnce({ data: { items: [] } })
    renderPage()
    expect(await screen.findByText(/no users yet/i)).toBeInTheDocument()
  })
})
