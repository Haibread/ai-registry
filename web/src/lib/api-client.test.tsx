import { describe, it, expect, vi } from 'vitest'
import { renderHook } from '@testing-library/react'

const authState = vi.hoisted(() => ({
  me: null as { user_id?: string } | null,
  isAuthenticated: false,
  authLoading: false,
}))
vi.mock('@/auth/AuthContext', () => ({ useAuth: () => authState }))

import { useCatalogClient } from './api-client'

describe('useCatalogClient', () => {
  it('is anonymous when signed out', () => {
    Object.assign(authState, { me: null, isAuthenticated: false, authLoading: false })
    const { result } = renderHook(() => useCatalogClient())
    expect(result.current.viewer).toBe('anonymous')
    expect(result.current.ready).toBe(true)
  })

  it('keys the cache by user when signed in', () => {
    Object.assign(authState, { me: { user_id: 'u1' }, isAuthenticated: true, authLoading: false })
    const { result } = renderHook(() => useCatalogClient())
    expect(result.current.viewer).toBe('u1')
  })

  it('is not ready while the session is loading', () => {
    Object.assign(authState, { me: null, isAuthenticated: false, authLoading: true })
    const { result } = renderHook(() => useCatalogClient())
    expect(result.current.ready).toBe(false)
  })
})
