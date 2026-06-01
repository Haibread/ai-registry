// AuthContext is the single source of auth *state* and the login/logout actions
// (ADR 0006 amendment, 2026-06-01). The browser holds no token — the registry
// session is an HttpOnly cookie — so "am I signed in?" is answered by a
// GET /api/v1/me fetch held here in state (not react-query), which keeps the
// global <Header> off the query layer. `usePermissions` (admin role gating)
// reads this `me`. OIDC is brokered server-side, so org sign-in is just a
// redirect to /api/v1/auth/oidc/login.
/* eslint-disable react-refresh/only-export-components */

import { createContext, useCallback, useContext, useEffect, useState } from 'react'
import type { components } from '@/lib/schema'

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
  login: () => void
  loginLocal: (email: string, password: string) => Promise<void>
  logout: () => Promise<void>
  refresh: () => Promise<void>
}

const AuthContext = createContext<AuthState | null>(null)

async function fetchMe(): Promise<Me | null> {
  try {
    const res = await fetch('/api/v1/me', {
      credentials: 'include',
      headers: { accept: 'application/json' },
    })
    if (!res.ok) return null
    return (await res.json()) as Me
  } catch {
    return null
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
    fetchMe().then((m) => {
      if (!cancelled) {
        setMe(m)
        setAuthLoading(false)
      }
    })
    return () => {
      cancelled = true
    }
  }, [])

  // A 401 on any request (from useAuthClient) dispatches this; re-check the
  // session so the UI flips to signed-out without a manual reload.
  useEffect(() => {
    const onUnauthorized = () => {
      void refresh()
    }
    window.addEventListener('auth:unauthorized', onUnauthorized)
    return () => window.removeEventListener('auth:unauthorized', onUnauthorized)
  }, [refresh])

  const login = useCallback(() => {
    window.location.href = '/api/v1/auth/oidc/login'
  }, [])

  const loginLocal = useCallback(
    async (email: string, password: string) => {
      setLoginError(null)
      const res = await fetch('/api/v1/auth/login', {
        method: 'POST',
        credentials: 'include',
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
      await refresh()
    },
    [refresh],
  )

  const logout = useCallback((): Promise<void> => {
    // Full navigation (not a background fetch): the server revokes the registry
    // session and, for an OIDC session, 302s through the IdP's RP-initiated
    // logout so the Keycloak SSO session ends too — otherwise re-clicking
    // "Sign in" would silently re-authenticate. A fetch could not follow that
    // cross-origin redirect. Local sessions just land back on the SPA.
    window.location.href = '/api/v1/auth/logout'
    return Promise.resolve()
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
