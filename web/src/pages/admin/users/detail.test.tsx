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

// Route-aware GET: the page loads the user AND the access aggregate.
function mockUserAndAccess(u: typeof user, access: { is_server_admin: boolean; items: unknown[] }) {
  mockGET.mockImplementation(async (path: string) =>
    path === '/api/v1/users/{id}/grants' ? { data: access } : { data: u },
  )
}

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
    mockUserAndAccess(user, { is_server_admin: false, items: [] })
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
    mockUserAndAccess({ ...user, is_server_admin: true }, { is_server_admin: true, items: [] })
    renderPage()
    await screen.findByText('Ada')
    expect(screen.getByRole('button', { name: /^disable account$/i })).toBeDisabled()
    expect(screen.getByRole('button', { name: /^revoke server admin$/i })).toBeDisabled()
  })

  it('shows the access grants with scope and group provenance', async () => {
    mockUserAndAccess(user, {
      is_server_admin: false,
      items: [
        { id: 'g1', role: 'viewer', publisher_id: 'p1', publisher_slug: 'acme', publisher_name: 'Acme Corp', source: 'api', via: 'direct', created_at: '2026-04-01T10:00:00Z' },
        { id: 'g2', role: 'editor', publisher_id: 'p1', publisher_slug: 'acme', publisher_name: 'Acme Corp', source: 'api', via: 'group', group_id: 'gr1', group_slug: 'acme-editors', group_name: 'Acme Editors', created_at: '2026-04-01T10:00:00Z' },
        { id: 'g3', role: 'reviewer', publisher_id: '', source: 'config', via: 'direct', created_at: '2026-04-01T10:00:00Z' },
      ],
    })
    renderPage()
    await screen.findByText('Ada')

    // Direct per-publisher grant: scope links to the publisher.
    const pubLinks = await screen.findAllByRole('link', { name: 'Acme Corp' })
    expect(pubLinks[0]).toHaveAttribute('href', '/admin/publishers/acme')
    // Group-derived grant names and links the group.
    expect(screen.getByRole('link', { name: 'Acme Editors' })).toHaveAttribute('href', '/admin/groups/acme-editors')
    // Global config grant: "All publishers" scope plus the config badge.
    expect(screen.getByText('All publishers')).toBeInTheDocument()
    expect(screen.getByText('config')).toBeInTheDocument()
  })

  it('shows the empty state when the user holds no grants', async () => {
    renderPage()
    await screen.findByText('Ada')
    expect(await screen.findByText(/no role grants/i)).toBeInTheDocument()
  })

  it('explains Server Admin access above the grants', async () => {
    mockUserAndAccess({ ...user, is_server_admin: true }, { is_server_admin: true, items: [] })
    renderPage()
    await screen.findByText('Ada')
    expect(await screen.findByText(/full access to every publisher/i)).toBeInTheDocument()
  })

  it('notes that IdP claim roles are not listed for federated users', async () => {
    mockUserAndAccess({ ...user, subject: 'oidc|123' }, { is_server_admin: false, items: [] })
    renderPage()
    await screen.findByText('Ada')
    expect(await screen.findByText(/matched at sign-in/i)).toBeInTheDocument()
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
