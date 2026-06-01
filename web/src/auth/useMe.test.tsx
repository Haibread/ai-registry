import { describe, it, expect, beforeEach, vi } from 'vitest'
import { renderHook } from '@testing-library/react'
import { usePermissions, satisfiesRole, type Role, type Me } from './useMe'

// usePermissions reads the cached identity from AuthContext
// — no react-query. Drive it via a mutable mocked useAuth.
let authState: { me: Me | null; authLoading: boolean; isAuthenticated: boolean }
vi.mock('@/auth/AuthContext', () => ({ useAuth: () => authState }))

function signedIn(me: Partial<Me>) {
  authState = {
    me: { authenticated: true, is_server_admin: false, grants: [], ...me } as Me,
    authLoading: false,
    isAuthenticated: true,
  }
}

beforeEach(() => {
  authState = { me: null, authLoading: false, isAuthenticated: false }
})

describe('satisfiesRole (mirrors the server lattice)', () => {
  const set = (...r: Role[]) => new Set<Role>(r)

  it('admin satisfies editor, viewer, and admin — but NOT reviewer', () => {
    expect(satisfiesRole(set('admin'), 'editor')).toBe(true)
    expect(satisfiesRole(set('admin'), 'viewer')).toBe(true)
    expect(satisfiesRole(set('admin'), 'admin')).toBe(true)
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
  it('scopes an editor to the publisher they hold a grant on', () => {
    signedIn({ grants: [{ role: 'editor', publisher_id: 'p1', publisher_slug: 'acme', publisher_name: 'Acme' }] })
    const { result } = renderHook(() => usePermissions())

    expect(result.current.grants.length).toBe(1)
    expect(result.current.isServerAdmin).toBe(false)
    expect(result.current.isEditorAnywhere).toBe(true)
    expect(result.current.isReviewerAnywhere).toBe(false)
    expect(result.current.canEdit('acme')).toBe(true)
    expect(result.current.canEdit('globex')).toBe(false)
    expect(result.current.canReview('acme')).toBe(false)
  })

  it('applies a global grant to every publisher', () => {
    signedIn({ grants: [{ role: 'reviewer' }] })
    const { result } = renderHook(() => usePermissions())

    expect(result.current.canReview('anything')).toBe(true)
    expect(result.current.canEdit('anything')).toBe(false)
    expect(result.current.isReviewerAnywhere).toBe(true)
  })

  it('lets a publisher admin edit and admin, but not review', () => {
    signedIn({ grants: [{ role: 'admin', publisher_id: 'p1', publisher_slug: 'acme', publisher_name: 'Acme' }] })
    const { result } = renderHook(() => usePermissions())

    expect(result.current.canEdit('acme')).toBe(true)
    expect(result.current.canAdmin('acme')).toBe(true)
    expect(result.current.isEditorAnywhere).toBe(true)
    expect(result.current.canReview('acme')).toBe(false)
    expect(result.current.isReviewerAnywhere).toBe(false)
  })

  it('lets a server admin do everything', () => {
    signedIn({ is_server_admin: true, grants: [] })
    const { result } = renderHook(() => usePermissions())

    expect(result.current.isServerAdmin).toBe(true)
    expect(result.current.canEdit('x')).toBe(true)
    expect(result.current.canReview('x')).toBe(true)
    expect(result.current.canAdmin('x')).toBe(true)
    expect(result.current.isEditorAnywhere).toBe(true)
  })
})
