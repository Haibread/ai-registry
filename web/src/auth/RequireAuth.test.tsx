// @vitest-environment jsdom

import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter, Routes, Route, Outlet } from 'react-router-dom'
import { RequireAuth } from './RequireAuth'

// RequireAuth derives auth state from useAuth (AuthContext's GET /api/v1/me),
// not a JS token. Mock it so we control the state.
vi.mock('./AuthContext', () => ({ useAuth: vi.fn() }))
import { useAuth } from './AuthContext'
const mockUseAuth = vi.mocked(useAuth)

function renderWithRouter(state: { isAuthenticated: boolean; authLoading: boolean }, initialPath = '/admin') {
  mockUseAuth.mockReturnValue(state as never)
  return render(
    <MemoryRouter initialEntries={[initialPath]}>
      <Routes>
        <Route path="/admin" element={<RequireAuth><Outlet /></RequireAuth>}>
          <Route index element={<div>Admin content</div>} />
        </Route>
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

  it('redirects to home when not authenticated', () => {
    renderWithRouter({ authLoading: false, isAuthenticated: false })
    expect(screen.getByText('Home')).toBeInTheDocument()
    expect(screen.queryByText(/admin content/i)).not.toBeInTheDocument()
  })

  it('renders children when authenticated', () => {
    renderWithRouter({ authLoading: false, isAuthenticated: true })
    expect(screen.getByText('Admin content')).toBeInTheDocument()
  })
})
