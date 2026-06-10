import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor, within } from '@testing-library/react'
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

// The page reads the caller's identity to block self-lockout actions.
let mockMyUserId = 'admin-self'
vi.mock('@/auth/useMe', () => ({
  useMe: () => ({ data: { user_id: mockMyUserId }, isLoading: false }),
}))

import AdminUserDetail from './detail'

// JSDOM doesn't implement HTMLDialogElement's modal methods; stub them so the
// ConfirmDialog opens and closes.
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
    mockMyUserId = 'admin-self'
    mockGET.mockResolvedValue({ data: user })
  })

  it('renders the user', async () => {
    renderPage()
    expect(await screen.findByText('Ada')).toBeInTheDocument()
    // Email appears in both the breadcrumb and the header.
    expect(screen.getAllByText('a@x.test').length).toBeGreaterThan(0)
  })

  it('toggles disabled via PATCH after confirmation', async () => {
    mockPATCH.mockResolvedValue({ error: undefined })
    renderPage()
    await screen.findByText('Ada')
    await userEvent.click(screen.getByRole('button', { name: /^disable account$/i }))
    // One click must not mutate — the confirmation names the user first.
    expect(mockPATCH).not.toHaveBeenCalled()
    const dialog = screen.getByRole('heading', { name: /disable account "a@x.test"\?/i }).closest('dialog')!
    await userEvent.click(within(dialog).getByRole('button', { name: /^disable account$/i }))
    await waitFor(() => {
      expect(mockPATCH).toHaveBeenCalledWith('/api/v1/users/{id}', {
        params: { path: { id: '01HU1' } },
        body: { disabled: true },
      })
    })
  })

  it('grants server admin via PATCH after confirmation', async () => {
    mockPATCH.mockResolvedValue({ error: undefined })
    renderPage()
    await screen.findByText('Ada')
    await userEvent.click(screen.getByRole('button', { name: /^grant server admin$/i }))
    expect(mockPATCH).not.toHaveBeenCalled()
    const dialog = screen.getByRole('heading', { name: /grant server admin to "a@x.test"\?/i }).closest('dialog')!
    await userEvent.click(within(dialog).getByRole('button', { name: /^grant server admin$/i }))
    await waitFor(() => {
      expect(mockPATCH).toHaveBeenCalledWith('/api/v1/users/{id}', {
        params: { path: { id: '01HU1' } },
        body: { is_server_admin: true },
      })
    })
  })

  it('cancelling the confirmation leaves the user untouched', async () => {
    mockPATCH.mockResolvedValue({ error: undefined })
    renderPage()
    await screen.findByText('Ada')
    await userEvent.click(screen.getByRole('button', { name: /^disable account$/i }))
    await userEvent.click(screen.getByRole('button', { name: /^cancel$/i }))
    expect(mockPATCH).not.toHaveBeenCalled()
  })

  it('blocks self-disable and self-demote (lockout protection)', async () => {
    mockMyUserId = '01HU1' // viewing my own page
    mockGET.mockResolvedValue({ data: { ...user, is_server_admin: true } })
    renderPage()
    await screen.findByText('Ada')
    expect(screen.getByRole('button', { name: /^disable account$/i })).toBeDisabled()
    expect(screen.getByRole('button', { name: /^revoke server admin$/i })).toBeDisabled()
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
