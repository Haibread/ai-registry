// Bearer-token store and refresh plumbing for the SPA.
//
// The registry is the single token authority: login mints a short-lived access
// token (Ed25519 JWT) plus a rotating, single-use refresh token. Both are
// persisted in localStorage so a page reload (or a new browser tab) can keep
// working WITHOUT spending the refresh token: on boot the SPA reuses the stored
// access token and only refreshes once it's actually rejected (a 401). This
// matters because the refresh token is single-use with server-side reuse
// detection — refreshing on every load would burn it needlessly.
//
// Trade-off: with no HttpOnly cookie, both tokens live in JS-reachable storage,
// so an XSS bug means token theft. The refresh token already being in
// localStorage is the dominant exposure (an attacker can mint access tokens at
// will), so persisting the access token too adds little. Mitigate with a short
// access TTL, rotating refresh tokens (reuse detection), and a strict CSP.
const ACCESS_KEY = 'ai_registry_access'
const REFRESH_KEY = 'ai_registry_refresh'

// In-memory mirrors, used as a fallback when localStorage is unavailable
// (private mode, partial test runtimes).
let memoryAccess: string | null = null
let memoryRefresh: string | null = null

function read(key: string, fallback: string | null): string | null {
  try {
    return localStorage.getItem(key) ?? fallback
  } catch {
    return fallback
  }
}

function write(key: string, value: string | null) {
  try {
    if (value) localStorage.setItem(key, value)
    else localStorage.removeItem(key)
  } catch {
    /* storage unavailable: the in-memory mirror is the source of truth */
  }
}

export function getAccessToken(): string | null {
  return read(ACCESS_KEY, memoryAccess)
}

export function getRefreshToken(): string | null {
  return read(REFRESH_KEY, memoryRefresh)
}

/** setTokens stores a freshly minted access + refresh pair. */
export function setTokens(access: string, refresh: string) {
  memoryAccess = access
  memoryRefresh = refresh
  write(ACCESS_KEY, access)
  write(REFRESH_KEY, refresh)
}

/** clearTokens drops both tokens (logout / unrecoverable 401). */
export function clearTokens() {
  memoryAccess = null
  memoryRefresh = null
  write(ACCESS_KEY, null)
  write(REFRESH_KEY, null)
}

// A single in-flight refresh is shared by all callers so a burst of 401s yields
// exactly one /auth/refresh round-trip.
let refreshInFlight: Promise<string | null> | null = null

/**
 * refreshAccessToken rotates the stored refresh token into a new access +
 * refresh pair and returns the new access token (or null when there is no usable
 * refresh token, i.e. the user must sign in again).
 */
export function refreshAccessToken(): Promise<string | null> {
  if (refreshInFlight) return refreshInFlight
  const rt = getRefreshToken()
  if (!rt) return Promise.resolve(null)

  refreshInFlight = (async () => {
    try {
      const res = await fetch('/api/v1/auth/refresh', {
        method: 'POST',
        headers: { 'content-type': 'application/json' },
        body: JSON.stringify({ refreshToken: rt }),
      })
      if (!res.ok) {
        clearTokens()
        return null
      }
      const body = (await res.json()) as { accessToken: string; refreshToken: string }
      setTokens(body.accessToken, body.refreshToken)
      return body.accessToken
    } catch {
      clearTokens()
      return null
    } finally {
      refreshInFlight = null
    }
  })()
  return refreshInFlight
}

/**
 * authFetch attaches the bearer access token and, on a 401, transparently
 * refreshes once and retries. Shared by the API client and the AuthContext
 * identity fetch so the refresh-on-401 logic lives in one place.
 *
 * Header handling is the subtle part: openapi-fetch calls this with a `Request`
 * object (method, body, and Content-Type baked into `input`, with `init`
 * undefined). Passing an `init.headers` alongside a Request REPLACES the
 * Request's headers, so we must seed our Headers from the Request first —
 * otherwise Content-Type is dropped and the server answers 415.
 */
export const authFetch: typeof fetch = async (input, init) => {
  const headers = new Headers(input instanceof Request ? input.headers : undefined)
  new Headers(init?.headers).forEach((value, key) => headers.set(key, value))
  const token = getAccessToken()
  if (token) headers.set('Authorization', `Bearer ${token}`)

  const res = await fetch(input, { ...init, headers })
  if (res.status !== 401) return res

  const fresh = await refreshAccessToken()
  if (!fresh) return res
  headers.set('Authorization', `Bearer ${fresh}`)
  return fetch(input, { ...init, headers })
}
