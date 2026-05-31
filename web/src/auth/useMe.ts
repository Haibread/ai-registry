import { useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import type { components } from '@/lib/schema'
import { useAuthClient } from '@/lib/api-client'
import { useAuth } from '@/auth/AuthContext'

// Types come straight from the generated OpenAPI client so the SPA's notion of
// a grant cannot drift from the server's (ADR 0006).
export type Me = components['schemas']['Me']
export type MeGrant = components['schemas']['MeGrant']
export type Role = MeGrant['role']

// admin implies editor (admins author) but NOT reviewer — reviewer is the sole
// approver, so only a reviewer grant (or the Server Admin break-glass) reviews.
const WRITE_ROLES: ReadonlySet<Role> = new Set<Role>(['editor', 'admin'])
const REVIEW_ROLES: ReadonlySet<Role> = new Set<Role>(['reviewer'])

/**
 * satisfiesRole mirrors the server's `domain.Satisfies` lattice: viewer is
 * implied by editor/reviewer/admin; admin implies editor (and admin) but NOT
 * reviewer — reviewer is the sole approver, so an Admin cannot approve. Editor
 * and reviewer stay independent (separation of duties). The UI uses this only
 * to decide what to render; the server is always the real gate (and the global
 * Server Admin break-glass is surfaced separately via `isServerAdmin`).
 */
export function satisfiesRole(held: ReadonlySet<Role>, required: Role): boolean {
  switch (required) {
    case 'viewer':
      return held.has('admin') || held.has('viewer') || held.has('editor') || held.has('reviewer')
    case 'editor':
      return held.has('admin') || held.has('editor')
    case 'reviewer':
      return held.has('reviewer') // admin does NOT satisfy reviewer
    case 'admin':
      return held.has('admin')
    default:
      return false
  }
}

/**
 * useMe fetches the caller's resolved identity + effective grants from
 * `GET /api/v1/me`. Enabled only when a token is present; cached for 5 minutes
 * since grants change rarely within a session.
 */
export function useMe() {
  const { accessToken } = useAuth()
  const api = useAuthClient()
  return useQuery({
    queryKey: ['me', accessToken ?? null],
    queryFn: async () => {
      const r = await api.GET('/api/v1/me')
      return r.data ?? null
    },
    enabled: !!accessToken,
    staleTime: 5 * 60_000,
  })
}

export interface Permissions {
  isLoading: boolean
  isAuthenticated: boolean
  isServerAdmin: boolean
  grants: MeGrant[]
  /** Effective roles the caller holds that apply to `slug` (incl. global grants). */
  rolesOn(slug: string | undefined): Set<Role>
  /** Can author/edit/deprecate on the given publisher. */
  canEdit(slug: string | undefined): boolean
  /** Can approve/reject on the given publisher. */
  canReview(slug: string | undefined): boolean
  /** Can manage the publisher (metadata, grants). */
  canAdmin(slug: string | undefined): boolean
  /** Can author on at least one publisher (or is Server Admin) — gates "New". */
  isEditorAnywhere: boolean
  /** Can review on at least one publisher (or is Server Admin). */
  isReviewerAnywhere: boolean
}

function rolesOn(grants: MeGrant[], slug: string | undefined): Set<Role> {
  const held = new Set<Role>()
  for (const g of grants) {
    const isGlobal = !g.publisher_slug
    if (isGlobal || g.publisher_slug === slug) held.add(g.role)
  }
  return held
}

/**
 * usePermissions derives a stable capability view from `useMe`. Components call
 * it to gate which actions/nav they render. Backed by the cached `me` query, so
 * many callers share one request. The server still enforces every write.
 */
export function usePermissions(): Permissions {
  const { data: me, isLoading } = useMe()
  const { accessToken } = useAuth()
  return useMemo<Permissions>(() => {
    const grants = me?.grants ?? []
    const isServerAdmin = me?.is_server_admin ?? false
    const anyOf = (roles: ReadonlySet<Role>) =>
      isServerAdmin || grants.some((g) => roles.has(g.role))
    return {
      isLoading,
      isAuthenticated: !!accessToken,
      isServerAdmin,
      grants,
      rolesOn: (slug) => rolesOn(grants, slug),
      canEdit: (slug) => isServerAdmin || satisfiesRole(rolesOn(grants, slug), 'editor'),
      canReview: (slug) => isServerAdmin || satisfiesRole(rolesOn(grants, slug), 'reviewer'),
      canAdmin: (slug) => isServerAdmin || satisfiesRole(rolesOn(grants, slug), 'admin'),
      isEditorAnywhere: anyOf(WRITE_ROLES),
      isReviewerAnywhere: anyOf(REVIEW_ROLES),
    }
  }, [me, isLoading, accessToken])
}
