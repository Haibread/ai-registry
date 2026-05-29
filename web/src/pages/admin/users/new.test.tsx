import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'

const mockNavigate = vi.fn()
vi.mock('react-router-dom', async (orig) => ({
  ...(await orig<typeof import('react-router-dom')>()),
  useNavigate: () => mockNavigate,
}))

const mockPOST = vi.fn()
vi.mock('@/lib/api-client', () => ({
  useAuthClient: () => ({ POST: mockPOST }),
}))

import AdminUserNew from './new'

function renderPage() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter>
        <AdminUserNew />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

describe('AdminUserNew', () => {
  beforeEach(() => vi.clearAllMocks())

  it('creates an invited user (no password) and navigates', async () => {
    mockPOST.mockResolvedValue({ data: { id: '01HU1' }, error: undefined })
    renderPage()

    await userEvent.type(screen.getByLabelText(/email/i), 'invitee@x.test')
    await userEvent.type(screen.getByLabelText(/display name/i), 'Invitee')
    await userEvent.click(screen.getByRole('button', { name: /create user/i }))

    await waitFor(() => {
      expect(mockPOST).toHaveBeenCalledWith('/api/v1/users', {
        body: { email: 'invitee@x.test', display_name: 'Invitee', password: undefined, is_server_admin: false },
      })
    })
    expect(mockNavigate).toHaveBeenCalledWith('/admin/users')
  })

  it('rejects a too-short password before calling the API', async () => {
    renderPage()
    await userEvent.type(screen.getByLabelText(/email/i), 'x@x.test')
    await userEvent.type(screen.getByLabelText(/^password/i), 'short')
    await userEvent.click(screen.getByRole('button', { name: /create user/i }))
    expect(await screen.findByRole('alert')).toHaveTextContent(/at least 8/i)
    expect(mockPOST).not.toHaveBeenCalled()
  })
})
