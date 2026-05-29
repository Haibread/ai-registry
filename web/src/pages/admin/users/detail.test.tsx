import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Routes, Route } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'

vi.mock('@/auth/AuthContext', () => ({
  useAuth: () => ({ accessToken: 'test-token' }),
}))

const mockGET = vi.fn()
const mockPATCH = vi.fn()
const mockPOST = vi.fn()
vi.mock('@/lib/api-client', () => ({
  useAuthClient: () => ({ GET: mockGET, PATCH: mockPATCH, POST: mockPOST }),
}))
vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }))

import AdminUserDetail from './detail'

const user = { id: '01HU1', email: 'a@x.test', display_name: 'Ada', subject: '', has_password: false, is_server_admin: false, disabled: false, created_at: '2026-04-01T10:00:00Z', updated_at: '2026-04-01T10:00:00Z' }

function renderPage() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={['/admin/users/01HU1']}>
        <Routes>
          <Route path="/admin/users/:id" element={<AdminUserDetail />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

describe('AdminUserDetail', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockGET.mockResolvedValue({ data: user })
  })

  it('renders the user', async () => {
    renderPage()
    expect(await screen.findByText('Ada')).toBeInTheDocument()
    // Email appears in both the breadcrumb and the header.
    expect(screen.getAllByText('a@x.test').length).toBeGreaterThan(0)
  })

  it('toggles disabled via PATCH', async () => {
    mockPATCH.mockResolvedValue({ error: undefined })
    renderPage()
    await screen.findByText('Ada')
    await userEvent.click(screen.getByRole('button', { name: /disable account/i }))
    await waitFor(() => {
      expect(mockPATCH).toHaveBeenCalledWith('/api/v1/users/{id}', {
        params: { path: { id: '01HU1' } },
        body: { disabled: true },
      })
    })
  })

  it('grants server admin via PATCH', async () => {
    mockPATCH.mockResolvedValue({ error: undefined })
    renderPage()
    await screen.findByText('Ada')
    await userEvent.click(screen.getByRole('button', { name: /grant server admin/i }))
    await waitFor(() => {
      expect(mockPATCH).toHaveBeenCalledWith('/api/v1/users/{id}', {
        params: { path: { id: '01HU1' } },
        body: { is_server_admin: true },
      })
    })
  })

  it('sets a password via POST', async () => {
    mockPOST.mockResolvedValue({ error: undefined })
    renderPage()
    await screen.findByText('Ada')
    await userEvent.type(screen.getByLabelText(/new password/i), 'longenough1')
    await userEvent.click(screen.getByRole('button', { name: /^set password$/i }))
    await waitFor(() => {
      expect(mockPOST).toHaveBeenCalledWith('/api/v1/users/{id}/set-password', {
        params: { path: { id: '01HU1' } },
        body: { password: 'longenough1' },
      })
    })
  })
})
