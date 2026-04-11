// @vitest-environment jsdom
//
// Tests for useAuthClient middleware:
//   - 401 responses trigger clearSession() so the UI shows Sign In immediately
//   - Authorization header is set when a token is present
//   - Authorization header is absent when no token

import { describe, it, expect, vi, beforeEach } from 'vitest'
import { renderHook } from '@testing-library/react'

const mockClearSession = vi.fn()

vi.mock('@/auth/AuthContext', () => ({
  useAuth: vi.fn(() => ({
    accessToken: 'test-token',
    clearSession: mockClearSession,
  })),
}))

// Capture the middleware registered by useAuthClient so we can call it directly,
// avoiding the jsdom relative-URL parsing issue that would occur with a real fetch.
type Middleware = {
  onRequest?: (ctx: { request: Request }) => Promise<Request>
  onResponse?: (ctx: { response: Response }) => Promise<Response>
}
let capturedMiddleware: Middleware = {}
const mockClient = {
  use: vi.fn((mw: Middleware) => { capturedMiddleware = mw }),
  GET: vi.fn(),
}
vi.mock('openapi-fetch', () => ({
  default: vi.fn(() => mockClient),
}))

import { useAuthClient } from './api-client'
import { useAuth } from '@/auth/AuthContext'

const mockUseAuth = vi.mocked(useAuth)

beforeEach(() => {
  vi.clearAllMocks()
  capturedMiddleware = {}
  mockUseAuth.mockReturnValue({
    accessToken: 'test-token',
    clearSession: mockClearSession,
    login: vi.fn(),
    logout: vi.fn(),
    user: null,
    isLoading: false,
    userManager: {} as never,
    loginError: null,
  })
})

describe('useAuthClient', () => {
  it('returns a client instance', () => {
    const { result } = renderHook(() => useAuthClient())
    expect(result.current).toBe(mockClient)
  })

  it('registers middleware on the client', () => {
    renderHook(() => useAuthClient())
    expect(mockClient.use).toHaveBeenCalledOnce()
    expect(capturedMiddleware.onRequest).toBeDefined()
    expect(capturedMiddleware.onResponse).toBeDefined()
  })

  it('calls clearSession when the server responds with 401', async () => {
    renderHook(() => useAuthClient())

    const response = new Response(JSON.stringify({ error: 'Unauthorized' }), { status: 401 })
    await capturedMiddleware.onResponse?.({ response })

    expect(mockClearSession).toHaveBeenCalledOnce()
  })

  it('does NOT call clearSession for non-401 responses', async () => {
    renderHook(() => useAuthClient())

    const response = new Response(JSON.stringify({ ok: true }), { status: 200 })
    await capturedMiddleware.onResponse?.({ response })

    expect(mockClearSession).not.toHaveBeenCalled()
  })

  it('sets Authorization header when accessToken is present', async () => {
    renderHook(() => useAuthClient())

    const request = new Request('http://localhost:3000/v1/stats')
    await capturedMiddleware.onRequest?.({ request })

    expect(request.headers.get('Authorization')).toBe('Bearer test-token')
  })

  it('does not set Authorization header when accessToken is absent', async () => {
    mockUseAuth.mockReturnValue({
      accessToken: undefined,
      clearSession: mockClearSession,
      login: vi.fn(),
      logout: vi.fn(),
      user: null,
      isLoading: false,
      userManager: {} as never,
      loginError: null,
    })

    renderHook(() => useAuthClient())

    const request = new Request('http://localhost:3000/v1/stats')
    await capturedMiddleware.onRequest?.({ request })

    expect(request.headers.get('Authorization')).toBeNull()
  })
})
