import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'

const mockNavigate = vi.fn()
vi.mock('react-router-dom', async (orig) => ({
  ...(await orig<typeof import('react-router-dom')>()),
  useNavigate: () => mockNavigate,
}))

const mockLogin = vi.fn()
const mockLoginLocal = vi.fn()
let authState: Record<string, unknown>
vi.mock('@/auth/AuthContext', () => ({
  useAuth: () => authState,
}))

import LoginPage from './login'

function renderPage(entry: string | { pathname: string; state?: unknown } = '/login') {
  return render(<MemoryRouter initialEntries={[entry]}><LoginPage /></MemoryRouter>)
}

describe('LoginPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    authState = {
      isAuthenticated: false,
      authLoading: false,
      configLoading: false,
      oidcEnabled: true,
      localLoginEnabled: true,
      login: mockLogin,
      loginLocal: mockLoginLocal,
    }
  })

  it('offers both the OIDC button and the local form', () => {
    renderPage()
    expect(screen.getByRole('button', { name: /sign in with your organization/i })).toBeInTheDocument()
    expect(screen.getByLabelText(/email/i)).toBeInTheDocument()
    expect(screen.getByLabelText(/password/i)).toBeInTheDocument()
  })

  it('triggers OIDC login on the org button', async () => {
    renderPage()
    await userEvent.click(screen.getByRole('button', { name: /sign in with your organization/i }))
    expect(mockLogin).toHaveBeenCalledOnce()
  })

  it('submits the local form and navigates to /admin on success', async () => {
    mockLoginLocal.mockResolvedValue(undefined)
    renderPage()
    await userEvent.type(screen.getByLabelText(/email/i), 'admin@example.com')
    await userEvent.type(screen.getByLabelText(/password/i), 'hunter2hunter2')
    await userEvent.click(screen.getByRole('button', { name: /^sign in$/i }))
    await waitFor(() => {
      expect(mockLoginLocal).toHaveBeenCalledWith('admin@example.com', 'hunter2hunter2')
    })
    expect(mockNavigate).toHaveBeenCalledWith('/admin')
  })

  it('honors a returnTo deep link after local login', async () => {
    mockLoginLocal.mockResolvedValue(undefined)
    renderPage({ pathname: '/login', state: { returnTo: '/admin/mcp/acme/thing' } })
    expect(screen.getByText(/sign in to continue to/i)).toBeInTheDocument()
    await userEvent.type(screen.getByLabelText(/email/i), 'admin@example.com')
    await userEvent.type(screen.getByLabelText(/password/i), 'hunter2hunter2')
    await userEvent.click(screen.getByRole('button', { name: /^sign in$/i }))
    await waitFor(() => expect(mockNavigate).toHaveBeenCalledWith('/admin/mcp/acme/thing'))
  })

  it('passes returnTo to the OIDC login so the round-trip can resume it', async () => {
    renderPage({ pathname: '/login', state: { returnTo: '/admin/users' } })
    await userEvent.click(screen.getByRole('button', { name: /sign in with your organization/i }))
    expect(mockLogin).toHaveBeenCalledWith('/admin/users')
  })

  it('rejects an external returnTo (no open redirect)', async () => {
    mockLoginLocal.mockResolvedValue(undefined)
    renderPage({ pathname: '/login', state: { returnTo: '//evil.example/phish' } })
    expect(screen.queryByText(/sign in to continue to/i)).not.toBeInTheDocument()
    await userEvent.type(screen.getByLabelText(/email/i), 'admin@example.com')
    await userEvent.type(screen.getByLabelText(/password/i), 'hunter2hunter2')
    await userEvent.click(screen.getByRole('button', { name: /^sign in$/i }))
    await waitFor(() => expect(mockNavigate).toHaveBeenCalledWith('/admin'))
  })

  it('shows the error message when local login fails', async () => {
    mockLoginLocal.mockRejectedValue(new Error('Invalid email or password.'))
    renderPage()
    await userEvent.type(screen.getByLabelText(/email/i), 'admin@example.com')
    await userEvent.type(screen.getByLabelText(/password/i), 'wrong')
    await userEvent.click(screen.getByRole('button', { name: /^sign in$/i }))
    expect(await screen.findByRole('alert')).toHaveTextContent(/invalid email or password/i)
    expect(mockNavigate).not.toHaveBeenCalled()
  })
})
