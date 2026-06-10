// AuthContext is the single source of auth *state* and the login/logout actions.
// Auth is a registry-issued bearer token (access + refresh): the SPA holds the
// access token in memory and the refresh token in storage (see auth/tokens.ts),
// and attaches `Authorization: Bearer …` on every API call. "Am I signed in?"
// is answered by a GET /api/v1/me fetch held here in state (not react-query),
// which keeps the global <Header> off the query layer. `usePermissions` (admin
// role gating) reads this `me`. OIDC is brokered server-side: org sign-in is a
// redirect to /api/v1/auth/oidc/login; the callback bounces back with a one-time
// handoff code in the URL fragment, which we exchange for tokens on load.
/* eslint-disable react-refresh/only-export-components */

import { createContext, useCallback, useContext, useEffect, useState } from 'react'
import type { components } from '@/lib/schema'
import { authFetch, clearTokens, getAccessToken, getRefreshToken, refreshAccessToken, setTokens } from '@/auth/tokens'

export type Me = components['schemas']['Me']

interface AppConfig {
  oidc_enabled: boolean
  local_login_enabled: boolean
}

interface AuthState {
  me: Me | null
  isAuthenticated: boolean
  authLoading: boolean
  configLoading: boolean
  oidcEnabled: boolean
  localLoginEnabled: boolean
  loginError: string | null
  /** Start the OIDC sign-in redirect; returnTo is resumed after the round-trip. */
  login: (returnTo?: string) => void
  loginLocal: (email: string, password: string) => Promise<void>
  logout: () => Promise<void>
  refresh: () => Promise<void>
}

const AuthContext = createContext<AuthState | null>(null)

// fetchMe resolves the caller's identity using the bearer token (authFetch
// attaches it and refreshes once on a 401). Returns null when signed out.
async function fetchMe(): Promise<Me | null> {
  try {
    const res = await authFetch('/api/v1/me', { headers: { accept: 'application/json' } })
    if (!res.ok) return null
    return (await res.json()) as Me
  } catch {
    return null
  }
}

// bootstrapAuth runs the one-time boot sequence (consume an OIDC handoff code,
// else reuse/refresh the stored token) and resolves the caller's identity. It is
// memoised into a single shared promise so React StrictMode's double-invoke of
// the mount effect (and any remount) reuses the same in-flight work instead of
// racing a second run. Without this, the first run consumes the single-use
// handoff code but is cancelled, while the second run finds the code already
// stripped, fetches /me with no token yet, and wins setMe(null) — leaving valid
// tokens in storage but the UI signed out. Mirrors the refreshInFlight singleton
// in auth/tokens.ts.
let bootInFlight: Promise<Me | null> | null = null
function bootstrapAuth(): Promise<Me | null> {
  if (!bootInFlight) {
    bootInFlight = (async () => {
      try {
        if (!(await consumeOIDCHandoff())) {
          if (!getAccessToken() && getRefreshToken()) await refreshAccessToken()
        }
        return await fetchMe()
      } finally {
        // Reset once settled. StrictMode's two effect runs both grab this promise
        // synchronously on mount, well before it resolves, so they still share one
        // run; clearing it afterwards just lets a genuine remount re-resolve /me
        // (the handoff code is already spent, so consumeOIDCHandoff is a no-op).
        bootInFlight = null
      }
    })()
  }
  return bootInFlight
}

// Where to resume after a full-page OIDC round-trip. The IdP redirect leaves
// the SPA entirely, so router state can't carry the deep link — sessionStorage
// (per-tab, survives the redirect) bridges it.
const RETURN_TO_KEY = 'ai_registry_return_to'

/** stashReturnTo records an in-app path to land on after OIDC sign-in. */
export function stashReturnTo(path: string) {
  try {
    sessionStorage.setItem(RETURN_TO_KEY, path)
  } catch {
    // Storage unavailable (private mode hard-limits) — login still works,
    // only the deep link is lost.
  }
}

// takeReturnTo pops the stashed path. Only same-origin absolute paths are
// honored so a crafted value can never turn this into an open redirect.
function takeReturnTo(): string | null {
  try {
    const v = sessionStorage.getItem(RETURN_TO_KEY)
    sessionStorage.removeItem(RETURN_TO_KEY)
    return v && v.startsWith('/') && !v.startsWith('//') ? v : null
  } catch {
    return null
  }
}

// consumeOIDCHandoff looks for the one-time code the OIDC callback left in the
// URL fragment (`#code=…`), exchanges it for tokens, and strips it from the URL.
// Returns true when a code was successfully exchanged.
async function consumeOIDCHandoff(): Promise<boolean> {
  if (!window.location.hash.startsWith('#')) return false
  const params = new URLSearchParams(window.location.hash.slice(1))
  const code = params.get('code')
  if (!code) return false

  // Strip the fragment regardless of outcome so a reload can't replay it.
  window.history.replaceState(null, '', window.location.pathname + window.location.search)
  try {
    const res = await fetch('/api/v1/auth/oidc/exchange', {
      method: 'POST',
      headers: { 'content-type': 'application/json' },
      body: JSON.stringify({ code }),
    })
    if (!res.ok) return false
    const body = (await res.json()) as { accessToken: string; refreshToken: string }
    setTokens(body.accessToken, body.refreshToken)
    // Resume the deep link that triggered the sign-in, if any. This runs
    // before the router mounts (bootstrapAuth), so a replaceState is enough
    // for the router to render the original destination directly.
    const returnTo = takeReturnTo()
    if (returnTo) window.history.replaceState(null, '', returnTo)
    return true
  } catch {
    return false
  }
}

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [me, setMe] = useState<Me | null>(null)
  const [authLoading, setAuthLoading] = useState(true)
  const [cfg, setCfg] = useState<AppConfig | null>(null)
  const [configLoading, setConfigLoading] = useState(true)
  const [loginError, setLoginError] = useState<string | null>(null)

  const refresh = useCallback(async () => {
    setMe(await fetchMe())
  }, [])

  useEffect(() => {
    let cancelled = false
    fetch('/config.json')
      .then((r) => (r.ok ? (r.json() as Promise<AppConfig>) : Promise.reject(new Error(String(r.status)))))
      .then((c) => {
        if (!cancelled) setCfg(c)
      })
      .catch(() => {
        if (!cancelled) setCfg({ oidc_enabled: false, local_login_enabled: true })
      })
      .finally(() => {
        if (!cancelled) setConfigLoading(false)
      })

    // Bootstrap: consume an OIDC handoff code if present; otherwise reuse the
    // stored access token (authFetch refreshes it on the first 401). Only spend
    // the single-use refresh token up front when there is no access token at all
    // — refreshing on every load would needlessly burn it. Memoised so the
    // single-use handoff code survives StrictMode's double-invoke (see
    // bootstrapAuth).
    void bootstrapAuth().then((m) => {
      if (!cancelled) {
        setMe(m)
        setAuthLoading(false)
      }
    })

    return () => {
      cancelled = true
    }
  }, [])

  // A 401 on any request (from useAuthClient) dispatches this; re-check identity
  // so the UI flips to signed-out without a manual reload.
  useEffect(() => {
    const onUnauthorized = () => {
      void refresh()
    }
    window.addEventListener('auth:unauthorized', onUnauthorized)
    return () => window.removeEventListener('auth:unauthorized', onUnauthorized)
  }, [refresh])

  const login = useCallback((returnTo?: string) => {
    if (returnTo) stashReturnTo(returnTo)
    window.location.href = '/api/v1/auth/oidc/login'
  }, [])

  const loginLocal = useCallback(
    async (email: string, password: string) => {
      setLoginError(null)
      const res = await fetch('/api/v1/auth/login', {
        method: 'POST',
        headers: { 'content-type': 'application/json' },
        body: JSON.stringify({ email, password }),
      })
      if (!res.ok) {
        let detail = 'Invalid email or password.'
        if (res.status === 404) detail = 'Local login is not enabled on this server.'
        else if (res.status === 429) detail = 'Too many attempts. Try again later.'
        else {
          const body = (await res.json().catch(() => null)) as { detail?: string; title?: string } | null
          if (body?.detail) detail = body.detail
          else if (body?.title) detail = body.title
        }
        throw new Error(detail)
      }
      const body = (await res.json()) as { accessToken: string; refreshToken: string }
      setTokens(body.accessToken, body.refreshToken)
      await refresh()
    },
    [refresh],
  )

  const logout = useCallback(async (): Promise<void> => {
    // Revoke the refresh token server-side. For an OIDC session the server
    // returns the IdP's RP-initiated logout URL so the upstream SSO session ends
    // too — otherwise re-clicking "Sign in" would silently re-authenticate.
    const refreshToken = getRefreshToken()
    let oidcLogoutUrl = ''
    try {
      const res = await fetch('/api/v1/auth/logout', {
        method: 'POST',
        headers: { 'content-type': 'application/json' },
        body: JSON.stringify({ refreshToken }),
      })
      if (res.ok) {
        const body = (await res.json().catch(() => null)) as { oidcLogoutUrl?: string } | null
        oidcLogoutUrl = body?.oidcLogoutUrl ?? ''
      }
    } catch {
      /* best-effort: clear locally even if the revoke call fails */
    }
    clearTokens()
    setMe(null)
    if (oidcLogoutUrl) window.location.href = oidcLogoutUrl
  }, [])

  return (
    <AuthContext.Provider
      value={{
        me,
        isAuthenticated: !!me?.authenticated,
        authLoading,
        configLoading,
        oidcEnabled: !!cfg?.oidc_enabled,
        localLoginEnabled: cfg?.local_login_enabled ?? true,
        loginError,
        login,
        loginLocal,
        logout,
        refresh,
      }}
    >
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth() {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used inside AuthProvider')
  return ctx
}
