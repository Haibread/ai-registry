import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'

interface MockPub {
  currentSlug: string | null
  current: { slug: string; name: string; roles: string[] } | null
}
let mockPub: MockPub
vi.mock('@/auth/PublisherContext', () => ({ usePublisher: () => mockPub }))

interface MockPerms {
  canAdmin: (slug: string) => boolean
  isServerAdmin: boolean
}
let mockPerms: MockPerms
vi.mock('@/auth/useMe', () => ({ usePermissions: () => mockPerms }))

vi.mock('@/components/admin/grants-section', () => ({
  GrantsSection: ({ publisherSlug }: { publisherSlug?: string }) => (
    <div data-testid="grants">grants:{publisherSlug}</div>
  ),
}))

import AdminMembers from './members'

function renderPage() {
  return render(
    <MemoryRouter>
      <AdminMembers />
    </MemoryRouter>,
  )
}

beforeEach(() => {
  mockPub = { currentSlug: 'acme', current: { slug: 'acme', name: 'Acme', roles: ['admin'] } }
  mockPerms = { canAdmin: () => true, isServerAdmin: false }
})

describe('AdminMembers', () => {
  it('prompts to select a publisher when none is chosen', () => {
    mockPub = { currentSlug: null, current: null }
    renderPage()
    expect(screen.getByText(/select a publisher/i)).toBeInTheDocument()
    expect(screen.queryByTestId('grants')).not.toBeInTheDocument()
  })

  it('blocks a non-admin member', () => {
    mockPerms = { canAdmin: () => false, isServerAdmin: false }
    renderPage()
    expect(screen.getByText(/admin access required/i)).toBeInTheDocument()
    expect(screen.queryByTestId('grants')).not.toBeInTheDocument()
  })

  it('renders the grants section scoped to the publisher for an admin', () => {
    renderPage()
    expect(screen.getByRole('heading', { name: /members & roles/i })).toBeInTheDocument()
    expect(screen.getByTestId('grants').textContent).toBe('grants:acme')
  })
})
