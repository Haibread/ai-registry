// PublisherContext scopes the admin area to one publisher at a time. The list
// of publishers the caller can act in is derived from their effective grants
// (GET /api/v1/me, via usePermissions) — so it cannot drift from the server's
// authorization. The selection persists in localStorage; a Server Admin also
// gets an "All publishers" scope (currentSlug = null) for the global view.
/* eslint-disable react-refresh/only-export-components */

import { createContext, useCallback, useContext, useMemo, useState } from 'react'
import { usePermissions, type Role } from '@/auth/useMe'

export interface PublisherOption {
  slug: string
  name: string
  roles: Role[]
}

interface PublisherContextValue {
  /** Publishers the caller holds at least one grant on (global grants excluded). */
  publishers: PublisherOption[]
  isServerAdmin: boolean
  /** Selected publisher slug, or null for the All-publishers (Server Admin) scope. */
  currentSlug: string | null
  /** The resolved option for currentSlug, or null when scope is All / none. */
  current: PublisherOption | null
  setCurrent: (slug: string | null) => void
  isLoading: boolean
}

const STORAGE_KEY = 'ai-registry.current_publisher'
// Persisted marker for an explicit Server-Admin "All publishers" choice, so it
// is not mistaken for "unset" and snapped back to the first publisher.
const ALL_SENTINEL = '__all__'

const PublisherContext = createContext<PublisherContextValue | null>(null)

export function PublisherProvider({ children }: { children: React.ReactNode }) {
  const perms = usePermissions()

  const publishers = useMemo<PublisherOption[]>(() => {
    const bySlug = new Map<string, PublisherOption>()
    for (const g of perms.grants) {
      if (!g.publisher_slug) continue // global grant — not scoped to one publisher
      const existing = bySlug.get(g.publisher_slug)
      if (existing) {
        if (!existing.roles.includes(g.role)) existing.roles.push(g.role)
      } else {
        bySlug.set(g.publisher_slug, {
          slug: g.publisher_slug,
          name: g.publisher_name || g.publisher_slug,
          roles: [g.role],
        })
      }
    }
    return [...bySlug.values()].sort((a, b) => a.name.localeCompare(b.name))
  }, [perms.grants])

  // The user's explicit choice (a slug, the All sentinel, or null = never
  // chosen). Persisted; reconciled against the live publisher set on read.
  const [stored, setStored] = useState<string | null>(() => localStorage.getItem(STORAGE_KEY))

  // Derive the effective selection rather than reconciling via an effect (no
  // cascading setState): an explicit All wins for a Server Admin; a still-valid
  // stored slug wins; otherwise default to the first publisher (a member's
  // natural home), or null when there are none.
  const currentSlug = useMemo<string | null>(() => {
    if (stored === ALL_SENTINEL) return perms.isServerAdmin ? null : (publishers[0]?.slug ?? null)
    if (stored && publishers.some((p) => p.slug === stored)) return stored
    return publishers[0]?.slug ?? null
  }, [stored, publishers, perms.isServerAdmin])

  const setCurrent = useCallback((slug: string | null) => {
    const v = slug ?? ALL_SENTINEL
    setStored(v)
    localStorage.setItem(STORAGE_KEY, v)
  }, [])

  const value = useMemo<PublisherContextValue>(
    () => ({
      publishers,
      isServerAdmin: perms.isServerAdmin,
      currentSlug,
      current: publishers.find((p) => p.slug === currentSlug) ?? null,
      setCurrent,
      isLoading: perms.isLoading,
    }),
    [publishers, perms.isServerAdmin, perms.isLoading, currentSlug, setCurrent],
  )

  return <PublisherContext.Provider value={value}>{children}</PublisherContext.Provider>
}

export function usePublisher(): PublisherContextValue {
  const ctx = useContext(PublisherContext)
  if (!ctx) throw new Error('usePublisher must be used inside PublisherProvider')
  return ctx
}
