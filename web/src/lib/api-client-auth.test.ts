// @vitest-environment jsdom
//
// Tests for the useAuthClient onResponse middleware: a 401 on any request other
// than /me dispatches `auth:unauthorized` so AuthContext re-checks identity and
// the UI flips to signed-out. /me is skipped (AuthContext owns that fetch). Auth
// rides in the Authorization bearer header (attached by authFetch).

import { describe, it, expect, vi, beforeEach } from 'vitest'
import { renderHook } from '@testing-library/react'

type Middleware = {
  onResponse?: (ctx: { response: Response }) => Promise<Response>
}
let captured: Middleware = {}
const mockClient = {
  use: vi.fn((mw: Middleware) => {
    captured = mw
  }),
  GET: vi.fn(),
}
vi.mock('openapi-fetch', () => ({ default: vi.fn(() => mockClient) }))

import { useAuthClient } from './api-client'

function resp(status: number, url: string): Response {
  const r = new Response(null, { status })
  Object.defineProperty(r, 'url', { value: url })
  return r
}

beforeEach(() => {
  vi.clearAllMocks()
  captured = {}
})

describe('useAuthClient', () => {
  it('returns the client and registers an onResponse middleware', () => {
    const { result } = renderHook(() => useAuthClient())
    expect(result.current).toBe(mockClient)
    expect(captured.onResponse).toBeDefined()
  })

  it('dispatches auth:unauthorized on a 401 from a non-/me request', async () => {
    const spy = vi.fn()
    window.addEventListener('auth:unauthorized', spy)
    renderHook(() => useAuthClient())
    await captured.onResponse?.({ response: resp(401, 'http://localhost/api/v1/stats') })
    expect(spy).toHaveBeenCalledOnce()
    window.removeEventListener('auth:unauthorized', spy)
  })

  it('does NOT dispatch on a 401 from /me itself (AuthContext owns it)', async () => {
    const spy = vi.fn()
    window.addEventListener('auth:unauthorized', spy)
    renderHook(() => useAuthClient())
    await captured.onResponse?.({ response: resp(401, 'http://localhost/api/v1/me') })
    expect(spy).not.toHaveBeenCalled()
    window.removeEventListener('auth:unauthorized', spy)
  })

  it('does NOT dispatch on a 2xx response', async () => {
    const spy = vi.fn()
    window.addEventListener('auth:unauthorized', spy)
    renderHook(() => useAuthClient())
    await captured.onResponse?.({ response: resp(200, 'http://localhost/api/v1/stats') })
    expect(spy).not.toHaveBeenCalled()
    window.removeEventListener('auth:unauthorized', spy)
  })
})
