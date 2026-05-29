// `AuthContext.tsx` colocates the `AuthProvider` component with the
// `useAuth` hook, the `getUserManager` helper, and the `resetManagerForTesting`
// escape hatch. Splitting them into separate files would require updating ~20
// import sites across the app for a marginal HMR benefit (auth state is
// resolved once at app start; a full reload on edits here is acceptable).
/* eslint-disable react-refresh/only-export-components */

import { createContext, useCallback, useContext, useEffect, useState } from 'react'
import { UserManager, WebStorageStateStore, type User } from 'oidc-client-ts'

// ── Runtime config fetch ──────────────────────────────────────────────────────
// OIDC coordinates are not baked into the bundle at build time. Instead the SPA
// fetches /config.json from the server on first load. The result is cached for
// the lifetime of the page so the network call is made at most once.

interface AppConfig {
  oidc_issuer: string
  oidc_client_id: string
  // 'session' (default) scopes tokens to the current tab — the secure default.
  // 'local' is an E2E-only escape hatch so Playwright's storageState() can
  // capture the authenticated session; the server ships 'session' in prod.
  auth_storage?: 'session' | 'local'
}

let _managerPromise: Promise<UserManager> | undefined

// _resolvedStore is the Storage the OIDC UserManager was built with (session
// or local, per /config.json). Captured so the local-login token persists in
// the SAME store — same XSS-blast-radius posture as the OIDC token.
let _resolvedStore: Storage | undefined

// Key under which the registry-issued local-login token is persisted.
const LOCAL_TOKEN_KEY = 'ai-registry.local_token'

/** Resets the module-level UserManager cache. Only for use in tests. */
export function resetManagerForTesting() {
  _managerPromise = undefined
  _resolvedStore = undefined
}

function localTokenStore(): Storage {
  return _resolvedStore ?? window.sessionStorage
}

// jwtExpired reports whether a JWT's `exp` (seconds) is in the past. A token we
// can't parse is treated as expired so a corrupt value never sticks. A live but
// server-rejected token is still cleared by the api-client's 401 handler.
function jwtExpired(token: string): boolean {
  const parts = token.split('.')
  if (parts.length !== 3) return true
  try {
    const payload = JSON.parse(atob(parts[1].replace(/-/g, '+').replace(/_/g, '/'))) as { exp?: number }
    return typeof payload.exp !== 'number' || payload.exp * 1000 <= Date.now()
  } catch {
    return true
  }
}

export function getUserManager(): Promise<UserManager> {
  if (_managerPromise) return _managerPromise
  _managerPromise = fetch('/config.json')
    .then((res) => {
      if (!res.ok) throw new Error(`GET /config.json failed: ${res.status}`)
      return res.json() as Promise<AppConfig>
    })
    .then(
      ({ oidc_issuer, oidc_client_id, auth_storage }) => {
        // Default to sessionStorage so OIDC tokens are scoped to the current
        // tab. The server only returns 'local' when explicitly configured for
        // Playwright e2e runs (AUTH_STORAGE=local), where storageState() must
        // be able to capture the authenticated session.
        const store = auth_storage === 'local' ? window.localStorage : window.sessionStorage
        _resolvedStore = store
        return new UserManager({
          authority: oidc_issuer,
          client_id: oidc_client_id,
          redirect_uri: window.location.origin + '/auth/callback',
          post_logout_redirect_uri: window.location.origin,
          response_type: 'code',
          scope: 'openid profile email',
          automaticSilentRenew: true,
          userStore: new WebStorageStateStore({ store }),
        })
      },
    )
    .catch((err) => {
      // Reset so the next call retries rather than replaying the rejection.
      _managerPromise = undefined
      throw err
    })
  return _managerPromise
}

// ── Context ───────────────────────────────────────────────────────────────────

interface AuthState {
  user: User | null
  isLoading: boolean
  loginError: string | null
  accessToken: string | undefined
  // email of the signed-in principal: the OIDC profile email, or — for a
  // local-token session — the email decoded from the registry token.
  email: string | undefined
  login: () => void
  // loginLocal exchanges email + password for a registry-issued token via
  // POST /api/v1/auth/login and stores it. Rejects with a user-facing message
  // on failure (the caller renders it).
  loginLocal: (email: string, password: string) => Promise<void>
  logout: () => void
  clearSession: () => Promise<void>
  userManager: UserManager | null
}

function jwtEmail(token: string): string | undefined {
  const parts = token.split('.')
  if (parts.length !== 3) return undefined
  try {
    const payload = JSON.parse(atob(parts[1].replace(/-/g, '+').replace(/_/g, '/'))) as { email?: string }
    return payload.email
  } catch {
    return undefined
  }
}

const AuthContext = createContext<AuthState | null>(null)

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [um, setUm] = useState<UserManager | null>(null)
  const [user, setUser] = useState<User | null>(null)
  const [isLoading, setIsLoading] = useState(true)
  const [loginError, setLoginError] = useState<string | null>(null)
  const [localToken, setLocalToken] = useState<string | null>(null)

  const clearLocalToken = useCallback(() => {
    localTokenStore().removeItem(LOCAL_TOKEN_KEY)
    setLocalToken(null)
  }, [])

  // Step 1: resolve UserManager (triggers the /config.json fetch once).
  // Do NOT call setIsLoading(false) here — wait until Step 2 has loaded the
  // user from localStorage, otherwise RequireAuth sees isLoading=false with no
  // accessToken and incorrectly redirects to "/" before the stored session is read.
  useEffect(() => {
    getUserManager()
      .then((resolved) => { setUm(resolved) })
      .catch((err: unknown) => {
        const msg = err instanceof Error ? err.message : String(err)
        setLoginError(`Authentication configuration failed: ${msg}`)
        setIsLoading(false)
      })
  }, [])

  // Step 2: subscribe to auth events once the manager is ready.
  useEffect(() => {
    if (!um) return

    // Restore a persisted local-login token (independent of any OIDC user).
    // The store is resolved by now (set when getUserManager resolved). Discard
    // an expired token so a stale value never briefly appears authenticated.
    const stored = localTokenStore().getItem(LOCAL_TOKEN_KEY)
    if (stored && !jwtExpired(stored)) {
      setLocalToken(stored)
    } else if (stored) {
      localTokenStore().removeItem(LOCAL_TOKEN_KEY)
    }

    um.getUser()
      .then((u) => {
        setUser(u)
        setIsLoading(false)
      })
      .catch(() => setIsLoading(false))

    const onUserLoaded = (u: User) => setUser(u)
    const onUserUnloaded = () => setUser(null)
    um.events.addUserLoaded(onUserLoaded)
    um.events.addUserUnloaded(onUserUnloaded)

    return () => {
      um.events.removeUserLoaded(onUserLoaded)
      um.events.removeUserUnloaded(onUserUnloaded)
    }
  }, [um])

  const login = useCallback(() => {
    setLoginError(null)
    if (!um) {
      setLoginError('Authentication is not configured. Check that the server is reachable and /config.json is served correctly.')
      return
    }
    um.signinRedirect().catch((err: unknown) => {
      const msg = err instanceof Error ? err.message : String(err)
      setLoginError(
        msg.includes('Failed to fetch') || msg.includes('NetworkError') || msg.includes('CORS')
          ? 'Cannot reach the authentication server. Check your OIDC configuration and CORS settings.'
          : `Sign-in failed: ${msg}`,
      )
    })
  }, [um])

  const loginLocal = useCallback(async (email: string, password: string) => {
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
    const { access_token } = (await res.json()) as { access_token: string }
    localTokenStore().setItem(LOCAL_TOKEN_KEY, access_token)
    setLocalToken(access_token)
  }, [])

  // logout clears whichever session is active. A local-token session just drops
  // the token (no IdP round-trip); an OIDC session does the IdP sign-out.
  const logout = useCallback(() => {
    if (localToken) clearLocalToken()
    if (um && user) return um.signoutRedirect()
  }, [um, user, localToken, clearLocalToken])

  // clearSession is the 401 hook: drop both token sources so the UI returns to
  // the signed-out state without an IdP redirect.
  const clearSession = useCallback(async () => {
    clearLocalToken()
    await um?.removeUser()
  }, [um, clearLocalToken])

  return (
    <AuthContext.Provider
      value={{
        user,
        isLoading,
        loginError,
        // A local token takes precedence over the OIDC one when both exist.
        accessToken: localToken ?? user?.access_token,
        email: localToken ? jwtEmail(localToken) : user?.profile?.email,
        login,
        loginLocal,
        logout,
        clearSession,
        userManager: um,
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
