import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'

vi.mock('@/auth/AuthContext', () => ({
  useAuth: () => ({ accessToken: 'test-token' }),
}))

const mockGET = vi.fn()
const mockPOST = vi.fn()
const mockDELETE = vi.fn()
vi.mock('@/lib/api-client', () => ({
  useAuthClient: () => ({ GET: mockGET, POST: mockPOST, DELETE: mockDELETE }),
}))
vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }))

// Group grants are Server-Admin-only; usePermissions gates the UI. Default to
// Server Admin so the existing group-grant flows stay valid; individual tests
// flip it to exercise the publisher-Admin view.
let mockIsServerAdmin = true
vi.mock('@/auth/useMe', () => ({
  usePermissions: () => ({ isServerAdmin: mockIsServerAdmin }),
}))

import { GrantsSection } from './grants-section'

const groups = [{ id: '01HG1', slug: 'platform', name: 'Platform', created_at: '', updated_at: '' }]
const users = [{ id: '01HU1', email: 'dev@x.test', has_password: true, is_server_admin: false, disabled: false, created_at: '', updated_at: '' }]
const grant = { id: '01HGR1', principal_type: 'group', principal_id: '01HG1', principal_label: 'platform', publisher_id: 'p1', role: 'editor', source: 'api', created_at: '' }

function mountGET(grantsData: unknown) {
  mockGET.mockImplementation((path: string) => {
    if (path === '/api/v1/groups') return Promise.resolve({ data: { items: groups } })
    if (path === '/api/v1/users') return Promise.resolve({ data: { items: users } })
    return Promise.resolve({ data: grantsData }) // /grants or /publishers/{slug}/grants
  })
}

function renderSection(props: { publisherSlug?: string } = {}) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <GrantsSection {...props} />
    </QueryClientProvider>,
  )
}

describe('GrantsSection', () => {
  beforeEach(() => { vi.clearAllMocks(); mockIsServerAdmin = true })

  it('lists existing grants', async () => {
    mountGET({ items: [grant] })
    renderSection()
    // The revoke button's label uniquely identifies the rendered grant row.
    expect(await screen.findByRole('button', { name: /revoke editor from platform/i })).toBeInTheDocument()
  })

  it('adds a global grant via POST /grants', async () => {
    mountGET({ items: [] })
    mockPOST.mockResolvedValue({ error: undefined })
    renderSection()

    // Wait for the group options to load, then pick the group + role.
    await screen.findByRole('option', { name: 'platform' })
    await userEvent.selectOptions(screen.getByLabelText(/^group$/i), '01HG1')
    await userEvent.selectOptions(screen.getByLabelText(/role/i), 'reviewer')
    await userEvent.click(screen.getByRole('button', { name: /grant/i }))

    await waitFor(() => {
      expect(mockPOST).toHaveBeenCalledWith('/api/v1/grants', {
        body: { principal_type: 'group', principal_id: '01HG1', role: 'reviewer' },
      })
    })
  })

  it('adds a per-publisher grant via POST /publishers/{slug}/grants', async () => {
    mountGET({ items: [] })
    mockPOST.mockResolvedValue({ error: undefined })
    renderSection({ publisherSlug: 'acme' })

    await screen.findByRole('option', { name: 'platform' })
    await userEvent.selectOptions(screen.getByLabelText(/^group$/i), '01HG1')
    await userEvent.click(screen.getByRole('button', { name: /grant/i }))

    await waitFor(() => {
      expect(mockPOST).toHaveBeenCalledWith('/api/v1/publishers/{slug}/grants', {
        params: { path: { slug: 'acme' } },
        body: { principal_type: 'group', principal_id: '01HG1', role: 'editor' },
      })
    })
  })

  it('switches the principal list to users', async () => {
    mountGET({ items: [] })
    renderSection()
    await userEvent.selectOptions(await screen.findByLabelText(/^principal$/i), 'user')
    // The principal dropdown now offers the user's email.
    expect(await screen.findByRole('option', { name: 'dev@x.test' })).toBeInTheDocument()
  })

  it('revokes a grant via DELETE', async () => {
    mountGET({ items: [grant] })
    mockDELETE.mockResolvedValue({ error: undefined })
    renderSection()
    await userEvent.click(await screen.findByRole('button', { name: /revoke editor from platform/i }))
    await waitFor(() => {
      expect(mockDELETE).toHaveBeenCalledWith('/api/v1/grants/{id}', { params: { path: { id: '01HGR1' } } })
    })
  })

  describe('publisher Admin (not Server Admin)', () => {
    beforeEach(() => { mockIsServerAdmin = false })

    it('hides the Group principal option and shows the instance-admin hint', async () => {
      mountGET({ items: [] })
      renderSection({ publisherSlug: 'acme' })
      // Only the User option is offered; Group is reserved for Server Admins.
      await waitFor(() => {
        expect(screen.queryByRole('option', { name: 'Group' })).not.toBeInTheDocument()
      })
      expect(screen.getByRole('option', { name: 'User' })).toBeInTheDocument()
      expect(screen.getByText(/group grants are managed by your instance admin/i)).toBeInTheDocument()
    })

    it('does not render a revoke button on a group grant', async () => {
      mountGET({ items: [grant] }) // grant.principal_type === 'group'
      renderSection({ publisherSlug: 'acme' })
      // The group row renders, but its revoke affordance is withheld.
      await screen.findByText('platform')
      expect(screen.queryByRole('button', { name: /revoke editor from platform/i })).not.toBeInTheDocument()
    })

    it('still allows revoking a user grant', async () => {
      const userGrant = { ...grant, id: '01HGR2', principal_type: 'user', principal_id: '01HU1', principal_label: 'dev@x.test' }
      mountGET({ items: [userGrant] })
      mockDELETE.mockResolvedValue({ error: undefined })
      renderSection({ publisherSlug: 'acme' })
      await userEvent.click(await screen.findByRole('button', { name: /revoke editor from dev@x.test/i }))
      await waitFor(() => {
        expect(mockDELETE).toHaveBeenCalledWith('/api/v1/publishers/{slug}/grants/{id}', { params: { path: { slug: 'acme', id: '01HGR2' } } })
      })
    })
  })
})
