// @vitest-environment jsdom

import { describe, it, expect, vi, beforeEach } from 'vitest'
import {
  REFRESH_BACKOFF_MS,
  authFetch,
  clearTokens,
  getAccessToken,
  getRefreshToken,
  refreshAccessToken,
  setTokens,
} from './tokens'

// The test runtime's localStorage is only partially implemented, so we exercise
// the store through its own API (which falls back to an in-memory copy when
// localStorage is unavailable) rather than poking localStorage directly.
beforeEach(() => {
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
  clearTokens()
})

// A Web Locks stand-in that runs `beforeGrant` while this tab "waits" for the
// lock, i.e. while another tab holds it.
function stubLocks(beforeGrant: () => void = () => {}) {
  const request = vi.fn(async (_name: string, cb: () => Promise<unknown>) => {
    beforeGrant()
    return cb()
  })
  vi.stubGlobal('navigator', { ...navigator, locks: { request } })
  return request
}

describe('token store', () => {
  it('stores the access token and exposes the refresh token', () => {
    setTokens('acc', 'ref')
    expect(getAccessToken()).toBe('acc')
    expect(getRefreshToken()).toBe('ref')
  })

  it('clearTokens drops both tokens', () => {
    setTokens('acc', 'ref')
    clearTokens()
    expect(getAccessToken()).toBeNull()
    expect(getRefreshToken()).toBeNull()
  })
})

describe('refreshAccessToken', () => {
  it('returns null and makes no request when there is no refresh token', async () => {
    const fetchMock = vi.spyOn(globalThis, 'fetch')
    expect(await refreshAccessToken()).toBeNull()
    expect(fetchMock).not.toHaveBeenCalled()
  })

  it('rotates the stored pair and returns the new access token', async () => {
    setTokens('old-acc', 'old-ref')
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ accessToken: 'new-acc', refreshToken: 'new-ref' }), { status: 200 }),
    )
    expect(await refreshAccessToken()).toBe('new-acc')
    expect(getAccessToken()).toBe('new-acc')
    expect(getRefreshToken()).toBe('new-ref')
  })

  it('keeps the tokens on a network error', async () => {
    setTokens('old-acc', 'old-ref')
    vi.spyOn(globalThis, 'fetch').mockRejectedValue(new TypeError('Failed to fetch'))
    expect(await refreshAccessToken()).toBeNull()
    expect(getAccessToken()).toBe('old-acc')
    expect(getRefreshToken()).toBe('old-ref')
  })

  it.each([500, 502, 503, 408, 429])('keeps the tokens on a transient %i', async (status) => {
    setTokens('old-acc', 'old-ref')
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response(null, { status }))
    expect(await refreshAccessToken()).toBeNull()
    expect(getRefreshToken()).toBe('old-ref')
  })

  it.each([400, 401, 403, 404, 422])('clears the tokens on a definitive %i', async (status) => {
    setTokens('old-acc', 'old-ref')
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response(null, { status }))
    expect(await refreshAccessToken()).toBeNull()
    expect(getAccessToken()).toBeNull()
    expect(getRefreshToken()).toBeNull()
  })

  it('backs off after a transient failure instead of retrying on every 401', async () => {
    vi.useFakeTimers()
    try {
      setTokens('old-acc', 'old-ref')
      const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response(null, { status: 503 }))
      expect(await refreshAccessToken()).toBeNull()
      expect(await refreshAccessToken()).toBeNull()
      expect(fetchMock).toHaveBeenCalledTimes(1)

      vi.advanceTimersByTime(REFRESH_BACKOFF_MS)
      fetchMock.mockResolvedValue(
        new Response(JSON.stringify({ accessToken: 'new-acc', refreshToken: 'new-ref' }), { status: 200 }),
      )
      expect(await refreshAccessToken()).toBe('new-acc')
      expect(fetchMock).toHaveBeenCalledTimes(2)
    } finally {
      vi.useRealTimers()
    }
  })

  it('coalesces concurrent refreshes into a single request', async () => {
    setTokens('old-acc', 'old-ref')
    const fetchMock = vi
      .spyOn(globalThis, 'fetch')
      .mockResolvedValue(new Response(JSON.stringify({ accessToken: 'a', refreshToken: 'r' }), { status: 200 }))
    await Promise.all([refreshAccessToken(), refreshAccessToken(), refreshAccessToken()])
    expect(fetchMock).toHaveBeenCalledTimes(1)
  })
})

describe('refreshAccessToken across tabs', () => {
  it('uses the pair another tab rotated while this tab waited for the lock', async () => {
    setTokens('old-acc', 'old-ref')
    const request = stubLocks(() => setTokens('other-acc', 'other-ref'))
    const fetchMock = vi.spyOn(globalThis, 'fetch')
    expect(await refreshAccessToken()).toBe('other-acc')
    expect(request).toHaveBeenCalledWith('ai-registry-token-refresh', expect.any(Function))
    expect(fetchMock).not.toHaveBeenCalled()
    expect(getRefreshToken()).toBe('other-ref')
  })

  it('returns null without a request when another tab signed out meanwhile', async () => {
    setTokens('old-acc', 'old-ref')
    stubLocks(() => clearTokens())
    const fetchMock = vi.spyOn(globalThis, 'fetch')
    expect(await refreshAccessToken()).toBeNull()
    expect(fetchMock).not.toHaveBeenCalled()
  })

  it('rotates under the lock when no other tab did', async () => {
    setTokens('old-acc', 'old-ref')
    const request = stubLocks()
    const fetchMock = vi
      .spyOn(globalThis, 'fetch')
      .mockResolvedValue(new Response(JSON.stringify({ accessToken: 'a', refreshToken: 'r' }), { status: 200 }))
    await Promise.all([refreshAccessToken(), refreshAccessToken()])
    expect(request).toHaveBeenCalledTimes(1)
    expect(fetchMock).toHaveBeenCalledTimes(1)
    expect(getRefreshToken()).toBe('r')
  })

  it('refreshes without a lock when Web Locks is unavailable', async () => {
    setTokens('old-acc', 'old-ref')
    vi.stubGlobal('navigator', { ...navigator, locks: undefined })
    const fetchMock = vi
      .spyOn(globalThis, 'fetch')
      .mockResolvedValue(new Response(JSON.stringify({ accessToken: 'a', refreshToken: 'r' }), { status: 200 }))
    expect(await refreshAccessToken()).toBe('a')
    expect(fetchMock).toHaveBeenCalledTimes(1)
  })

  it('authFetch retries with the access token another tab minted', async () => {
    setTokens('stale', 'old-ref')
    stubLocks(() => setTokens('other-acc', 'other-ref'))
    const fetchMock = vi
      .spyOn(globalThis, 'fetch')
      .mockResolvedValueOnce(new Response(null, { status: 401 }))
      .mockResolvedValueOnce(new Response('ok', { status: 200 }))
    const res = await authFetch('/api/v1/thing')
    expect(res.status).toBe(200)
    expect(fetchMock).toHaveBeenCalledTimes(2)
    const retryInit = fetchMock.mock.calls[1][1] as RequestInit
    expect(new Headers(retryInit.headers).get('Authorization')).toBe('Bearer other-acc')
  })

  it('drops the in-memory copy when another tab clears the tokens', () => {
    setTokens('acc', 'ref')
    localStorage.removeItem('ai_registry_refresh')
    localStorage.removeItem('ai_registry_access')
    window.dispatchEvent(new StorageEvent('storage', { key: null }))
    expect(getRefreshToken()).toBeNull()
    expect(getAccessToken()).toBeNull()
  })
})

describe('authFetch', () => {
  it('attaches the bearer access token', async () => {
    setTokens('acc', 'ref')
    const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response(null, { status: 200 }))
    await authFetch('/api/v1/thing')
    const init = fetchMock.mock.calls[0][1] as RequestInit
    expect(new Headers(init.headers).get('Authorization')).toBe('Bearer acc')
  })

  it('preserves a Request object headers (openapi-fetch) while adding the bearer', async () => {
    // openapi-fetch calls fetch with a Request (Content-Type baked in, init
    // undefined). authFetch must merge those headers, not replace them.
    setTokens('acc', 'ref')
    const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response(null, { status: 200 }))
    const req = new Request('http://localhost/api/v1/x', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: '{}',
    })
    await authFetch(req)
    const init = fetchMock.mock.calls[0][1] as RequestInit
    const sent = new Headers(init.headers)
    expect(sent.get('Content-Type')).toBe('application/json')
    expect(sent.get('Authorization')).toBe('Bearer acc')
  })

  it('returns the original 401 and keeps the session when the refresh fails transiently', async () => {
    setTokens('stale', 'ref')
    const fetchMock = vi
      .spyOn(globalThis, 'fetch')
      .mockResolvedValueOnce(new Response(null, { status: 401 }))
      .mockRejectedValueOnce(new TypeError('Failed to fetch'))
    const res = await authFetch('/api/v1/thing')
    expect(res.status).toBe(401)
    expect(fetchMock).toHaveBeenCalledTimes(2)
    expect(getRefreshToken()).toBe('ref')
  })

  it('refreshes once on a 401 and retries the original request', async () => {
    setTokens('stale', 'ref')
    const fetchMock = vi
      .spyOn(globalThis, 'fetch')
      .mockResolvedValueOnce(new Response(null, { status: 401 })) // original request
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ accessToken: 'fresh', refreshToken: 'ref2' }), { status: 200 }),
      ) // /auth/refresh
      .mockResolvedValueOnce(new Response('ok', { status: 200 })) // retried request
    const res = await authFetch('/api/v1/thing')
    expect(res.status).toBe(200)
    // Third call is the retry, carrying the refreshed token.
    const retryInit = fetchMock.mock.calls[2][1] as RequestInit
    expect(new Headers(retryInit.headers).get('Authorization')).toBe('Bearer fresh')
  })

  it('retries a Request object (openapi-fetch) with its body intact after a 401', async () => {
    setTokens('stale', 'ref')
    const sent: { url: string; method: string; body: string; auth: string | null }[] = []
    const responses = [
      new Response(null, { status: 401 }),
      new Response(JSON.stringify({ accessToken: 'fresh', refreshToken: 'ref2' }), { status: 200 }),
      new Response('ok', { status: 200 }),
    ]
    // Consume the body like a real fetch does, so a reused Request is caught.
    vi.spyOn(globalThis, 'fetch').mockImplementation(async (input, init) => {
      const req = new Request(input instanceof Request ? input : new URL(String(input), 'http://localhost'), init)
      sent.push({ url: req.url, method: req.method, body: await req.text(), auth: req.headers.get('Authorization') })
      return responses.shift()!
    })
    const payload = JSON.stringify({ name: 'demo' })
    const req = new Request('http://localhost/api/v1/x', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: payload,
    })

    const res = await authFetch(req)

    expect(res.status).toBe(200)
    expect(sent).toHaveLength(3)
    expect(sent[2]).toEqual({ url: 'http://localhost/api/v1/x', method: 'POST', body: payload, auth: 'Bearer fresh' })
  })
})
