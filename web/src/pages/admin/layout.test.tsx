import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, within } from '@testing-library/react'
import { MemoryRouter, Routes, Route } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'

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

// The sidebar fetches the review-queue count via useAuthClient. Mock it
// out so layout tests stay focused on the layout shell.
vi.mock('@/lib/api-client', () => ({
  useAuthClient: () => ({ GET: vi.fn().mockResolvedValue({ data: { items: [] } }) }),
}))

import AdminLayout from './layout'

function renderLayout(initialPath = '/admin') {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={[initialPath]}>
        <Routes>
          <Route path="/admin" element={<AdminLayout />}>
            <Route index element={<div>child content</div>} />
            <Route path="mcp" element={<div>mcp child</div>} />
          </Route>
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
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

  describe('mobile drawer', () => {
    it('renders a hamburger toggle that opens and closes a navigation dialog', () => {
      renderLayout()
      const toggle = screen.getByRole('button', { name: /open navigation menu/i })
      expect(toggle).toHaveAttribute('aria-expanded', 'false')

      // Closed → no dialog rendered.
      expect(screen.queryByRole('dialog', { name: /admin navigation/i })).toBeNull()

      fireEvent.click(toggle)
      const dialog = screen.getByRole('dialog', { name: /admin navigation/i })
      expect(dialog).toBeInTheDocument()
      // Same button toggles closed (label flips to "Close…").
      expect(screen.getByRole('button', { name: /close navigation menu/i })).toHaveAttribute(
        'aria-expanded',
        'true',
      )

      // Click the toggle a second time → dialog goes away.
      fireEvent.click(screen.getByRole('button', { name: /close navigation menu/i }))
      expect(screen.queryByRole('dialog', { name: /admin navigation/i })).toBeNull()
    })

    it('closes the drawer when a nav link inside it is clicked', () => {
      renderLayout()
      fireEvent.click(screen.getByRole('button', { name: /open navigation menu/i }))
      const dialog = screen.getByRole('dialog', { name: /admin navigation/i })

      // Both the desktop sidebar and the drawer render the same nav links,
      // so we scope the click to within the drawer to mimic a real tap.
      const dashboardInDrawer = within(dialog).getByRole('link', { name: /dashboard/i })
      fireEvent.click(dashboardInDrawer)

      expect(screen.queryByRole('dialog', { name: /admin navigation/i })).toBeNull()
    })
  })
})
