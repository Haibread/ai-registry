import { describe, it, expect, vi, beforeEach } from 'vitest'
import { renderHook, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import type { ReactNode } from 'react'

vi.mock('@/auth/AuthContext', () => ({
  useAuth: () => ({ accessToken: 'test-token' }),
}))

const mockGET = vi.fn()
vi.mock('@/lib/api-client', () => ({
  useAuthClient: () => ({ GET: mockGET }),
}))

import { usePermissions, satisfiesRole, type Role } from './useMe'

function makeWrapper() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={qc}>{children}</QueryClientProvider>
  )
}

describe('satisfiesRole (mirrors the server lattice, ADR 0006)', () => {
  const set = (...r: Role[]) => new Set<Role>(r)

  it('admin satisfies editor, viewer, and admin — but NOT reviewer', () => {
    expect(satisfiesRole(set('admin'), 'editor')).toBe(true)
    expect(satisfiesRole(set('admin'), 'viewer')).toBe(true)
    expect(satisfiesRole(set('admin'), 'admin')).toBe(true)
    // Reviewer is the sole approver — admin cannot approve.
    expect(satisfiesRole(set('admin'), 'reviewer')).toBe(false)
    expect(satisfiesRole(set('admin', 'reviewer'), 'reviewer')).toBe(true)
  })

  it('keeps editor and reviewer independent (separation of duties)', () => {
    expect(satisfiesRole(set('editor'), 'reviewer')).toBe(false)
    expect(satisfiesRole(set('reviewer'), 'editor')).toBe(false)
  })

  it('implies viewer from editor or reviewer', () => {
    expect(satisfiesRole(set('editor'), 'viewer')).toBe(true)
    expect(satisfiesRole(set('reviewer'), 'viewer')).toBe(true)
    expect(satisfiesRole(set(), 'viewer')).toBe(false)
  })
})

describe('usePermissions', () => {
  beforeEach(() => vi.clearAllMocks())

  it('scopes an editor to the publisher they hold a grant on', async () => {
    mockGET.mockResolvedValue({
      data: {
        authenticated: true,
        is_server_admin: false,
        grants: [{ role: 'editor', publisher_id: 'p1', publisher_slug: 'acme', publisher_name: 'Acme' }],
      },
    })
    const { result } = renderHook(() => usePermissions(), { wrapper: makeWrapper() })
    await waitFor(() => expect(result.current.grants.length).toBe(1))

    expect(result.current.isServerAdmin).toBe(false)
    expect(result.current.isEditorAnywhere).toBe(true)
    expect(result.current.isReviewerAnywhere).toBe(false)
    expect(result.current.canEdit('acme')).toBe(true)
    expect(result.current.canEdit('globex')).toBe(false)
    expect(result.current.canReview('acme')).toBe(false)
  })

  it('applies a global grant to every publisher', async () => {
    mockGET.mockResolvedValue({
      data: { authenticated: true, is_server_admin: false, grants: [{ role: 'reviewer' }] },
    })
    const { result } = renderHook(() => usePermissions(), { wrapper: makeWrapper() })
    await waitFor(() => expect(result.current.grants.length).toBe(1))

    expect(result.current.canReview('anything')).toBe(true)
    expect(result.current.canEdit('anything')).toBe(false)
    expect(result.current.isReviewerAnywhere).toBe(true)
  })

  it('lets a publisher admin edit and admin, but not review', async () => {
    mockGET.mockResolvedValue({
      data: {
        authenticated: true,
        is_server_admin: false,
        grants: [{ role: 'admin', publisher_id: 'p1', publisher_slug: 'acme', publisher_name: 'Acme' }],
      },
    })
    const { result } = renderHook(() => usePermissions(), { wrapper: makeWrapper() })
    await waitFor(() => expect(result.current.grants.length).toBe(1))

    expect(result.current.canEdit('acme')).toBe(true)
    expect(result.current.canAdmin('acme')).toBe(true)
    expect(result.current.isEditorAnywhere).toBe(true)
    // Admin is not an approver — only a Reviewer grant (or Server Admin) is.
    expect(result.current.canReview('acme')).toBe(false)
    expect(result.current.isReviewerAnywhere).toBe(false)
  })

  it('lets a server admin do everything', async () => {
    mockGET.mockResolvedValue({
      data: { authenticated: true, is_server_admin: true, grants: [] },
    })
    const { result } = renderHook(() => usePermissions(), { wrapper: makeWrapper() })
    await waitFor(() => expect(result.current.isServerAdmin).toBe(true))

    expect(result.current.canEdit('x')).toBe(true)
    expect(result.current.canReview('x')).toBe(true)
    expect(result.current.canAdmin('x')).toBe(true)
    expect(result.current.isEditorAnywhere).toBe(true)
  })
})
