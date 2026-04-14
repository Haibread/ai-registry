import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { MemoryRouter, Routes, Route } from 'react-router-dom'

vi.mock('@/components/layout/theme-toggle', () => ({
  ThemeToggle: () => <button type="button">Toggle theme</button>,
}))

const mockLogout = vi.fn()
vi.mock('@/auth/AuthContext', () => ({
  useAuth: () => ({
    accessToken: 'test-token',
    user: { profile: { email: 'admin@example.com', preferred_username: 'admin' } },
    logout: mockLogout,
    clearSession: vi.fn(),
  }),
}))

import AdminLayout from './layout'

function renderLayout(initialPath = '/admin') {
  return render(
    <MemoryRouter initialEntries={[initialPath]}>
      <Routes>
        <Route path="/admin" element={<AdminLayout />}>
          <Route index element={<div>child content</div>} />
          <Route path="mcp" element={<div>mcp child</div>} />
        </Route>
      </Routes>
    </MemoryRouter>,
  )
}

describe('AdminLayout', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders the Registry / Admin header', () => {
    renderLayout()
    expect(screen.getByText(/registry/i)).toBeInTheDocument()
    expect(screen.getByText(/^admin$/i)).toBeInTheDocument()
  })

  it('renders the user email from the auth profile', () => {
    renderLayout()
    expect(screen.getByText('admin@example.com')).toBeInTheDocument()
  })

  it('renders sidebar nav items', () => {
    renderLayout()
    expect(screen.getByRole('link', { name: /dashboard/i })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /publishers/i })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /mcp servers/i })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /agents/i })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /reports/i })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /api keys/i })).toBeInTheDocument()
  })

  it('renders the routed child content inside the main outlet', () => {
    renderLayout('/admin')
    expect(screen.getByText('child content')).toBeInTheDocument()
  })

  it('calls logout when the Sign out button is clicked', () => {
    renderLayout()
    fireEvent.click(screen.getByRole('button', { name: /sign out/i }))
    expect(mockLogout).toHaveBeenCalledTimes(1)
  })
})
