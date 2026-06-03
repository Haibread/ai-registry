// @vitest-environment jsdom

import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter, Routes, Route } from 'react-router-dom'
import { RequireServerAdmin } from './RequireServerAdmin'

// The guard reads usePermissions (derived from AuthContext's GET /api/v1/me).
// Mock it so we control isServerAdmin / isLoading.
vi.mock('@/auth/useMe', () => ({ usePermissions: vi.fn() }))
import { usePermissions } from '@/auth/useMe'
const mockPerms = vi.mocked(usePermissions)

function renderAt(state: { isLoading: boolean; isServerAdmin: boolean }) {
  mockPerms.mockReturnValue(state as never)
  return render(
    <MemoryRouter initialEntries={['/admin/users']}>
      <Routes>
        <Route path="/admin" element={<div>Admin dashboard</div>} />
        <Route element={<RequireServerAdmin />}>
          <Route path="/admin/users" element={<div>Users management</div>} />
        </Route>
      </Routes>
    </MemoryRouter>,
  )
}

describe('RequireServerAdmin', () => {
  it('shows a spinner while permissions load', () => {
    renderAt({ isLoading: true, isServerAdmin: false })
    expect(screen.getByText(/loading/i)).toBeInTheDocument()
    expect(screen.queryByText(/users management/i)).not.toBeInTheDocument()
  })

  it('redirects a non-Server-Admin to the dashboard', () => {
    renderAt({ isLoading: false, isServerAdmin: false })
    expect(screen.getByText('Admin dashboard')).toBeInTheDocument()
    expect(screen.queryByText(/users management/i)).not.toBeInTheDocument()
  })

  it('renders the guarded route for a Server Admin', () => {
    renderAt({ isLoading: false, isServerAdmin: true })
    expect(screen.getByText('Users management')).toBeInTheDocument()
  })
})
