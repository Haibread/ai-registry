// @vitest-environment jsdom

import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter, Routes, Route, Outlet } from 'react-router-dom'
import { RequireAuth } from './RequireAuth'

// Mock AuthContext so we control auth state without oidc-client-ts
vi.mock('./AuthContext', () => ({
  useAuth: vi.fn(),
}))

import { useAuth } from './AuthContext'
const mockUseAuth = vi.mocked(useAuth)

const mockLogin = vi.fn()

function renderWithRouter(authState: ReturnType<typeof useAuth>, initialPath = '/admin') {
  mockUseAuth.mockReturnValue(authState)
  return render(
    <MemoryRouter initialEntries={[initialPath]}>
      <Routes>
        <Route path="/admin" element={<RequireAuth><Outlet /></RequireAuth>}>
          <Route index element={<div>Admin content</div>} />
        </Route>
        <Route path="/" element={<div>Home</div>} />
      </Routes>
    </MemoryRouter>
  )
}

describe('RequireAuth', () => {
  it('shows spinner while loading', () => {
    renderWithRouter({ isLoading: true, accessToken: undefined, login: mockLogin, logout: vi.fn(), clearSession: vi.fn(), user: null, userManager: {} as never })
    expect(screen.getByText(/loading/i)).toBeInTheDocument()
  })

  it('redirects to home page (not Keycloak) when not authenticated', () => {
    renderWithRouter({ isLoading: false, accessToken: undefined, login: mockLogin, logout: vi.fn(), clearSession: vi.fn(), user: null, userManager: {} as never })
    // Should navigate to "/" and render Home, not call login() or show Keycloak redirect
    expect(mockLogin).not.toHaveBeenCalled()
    expect(screen.getByText('Home')).toBeInTheDocument()
    expect(screen.queryByText(/admin content/i)).not.toBeInTheDocument()
  })

  it('renders children when authenticated', () => {
    renderWithRouter({ isLoading: false, accessToken: 'tok-abc', login: mockLogin, logout: vi.fn(), clearSession: vi.fn(), user: null, userManager: {} as never })
    expect(screen.getByText('Admin content')).toBeInTheDocument()
  })
})
