// @vitest-environment jsdom

import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter, Routes, Route, Outlet, useLocation } from 'react-router-dom'
import { RequireAuth } from './RequireAuth'

// RequireAuth derives auth state from useAuth (AuthContext's GET /api/v1/me),
// not a JS token. Mock it so we control the state.
vi.mock('./AuthContext', () => ({ useAuth: vi.fn() }))
import { useAuth } from './AuthContext'
const mockUseAuth = vi.mocked(useAuth)

// LoginProbe surfaces the router state RequireAuth attached to the redirect.
function LoginProbe() {
  const location = useLocation()
  const returnTo = (location.state as { returnTo?: string } | null)?.returnTo
  return <div>Login page (returnTo: {returnTo ?? 'none'})</div>
}

function renderWithRouter(state: { isAuthenticated: boolean; authLoading: boolean }, initialPath = '/admin') {
  mockUseAuth.mockReturnValue(state as never)
  return render(
    <MemoryRouter initialEntries={[initialPath]}>
      <Routes>
        <Route path="/admin" element={<RequireAuth><Outlet /></RequireAuth>}>
          <Route index element={<div>Admin content</div>} />
          <Route path="mcp/acme/thing" element={<div>Entry detail</div>} />
        </Route>
        <Route path="/login" element={<LoginProbe />} />
        <Route path="/" element={<div>Home</div>} />
      </Routes>
    </MemoryRouter>,
  )
}

describe('RequireAuth', () => {
  it('shows spinner while loading', () => {
    renderWithRouter({ authLoading: true, isAuthenticated: false })
    expect(screen.getByText(/loading/i)).toBeInTheDocument()
  })

  it('redirects to the login page when not authenticated', () => {
    renderWithRouter({ authLoading: false, isAuthenticated: false })
    expect(screen.getByText(/login page/i)).toBeInTheDocument()
    expect(screen.queryByText(/admin content/i)).not.toBeInTheDocument()
  })

  it('carries the original deep link as returnTo state', () => {
    renderWithRouter({ authLoading: false, isAuthenticated: false }, '/admin/mcp/acme/thing')
    expect(screen.getByText('Login page (returnTo: /admin/mcp/acme/thing)')).toBeInTheDocument()
  })

  it('renders children when authenticated', () => {
    renderWithRouter({ authLoading: false, isAuthenticated: true })
    expect(screen.getByText('Admin content')).toBeInTheDocument()
  })
})
