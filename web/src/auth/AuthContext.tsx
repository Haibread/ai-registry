import { createContext, useCallback, useContext, useEffect, useState } from 'react'
import { UserManager, type User } from 'oidc-client-ts'

// ── Runtime config fetch ──────────────────────────────────────────────────────
// OIDC coordinates are not baked into the bundle at build time. Instead the SPA
// fetches /config.json from the server on first load. The result is cached for
// the lifetime of the page so the network call is made at most once.

interface AppConfig {
  oidc_issuer: string
  oidc_client_id: string
}

let _managerPromise: Promise<UserManager> | undefined

export function getUserManager(): Promise<UserManager> {
  if (_managerPromise) return _managerPromise
  _managerPromise = fetch('/config.json')
    .then((res) => {
      if (!res.ok) throw new Error(`GET /config.json failed: ${res.status}`)
      return res.json() as Promise<AppConfig>
    })
    .then(
      ({ oidc_issuer, oidc_client_id }) =>
        new UserManager({
          authority: oidc_issuer,
          client_id: oidc_client_id,
          redirect_uri: window.location.origin + '/auth/callback',
          post_logout_redirect_uri: window.location.origin,
          response_type: 'code',
          scope: 'openid profile email',
          automaticSilentRenew: true,
        }),
    )
  return _managerPromise
}

// ── Context ───────────────────────────────────────────────────────────────────

interface AuthState {
  user: User | null
  isLoading: boolean
  loginError: string | null
  accessToken: string | undefined
  login: () => void
  logout: () => void
  clearSession: () => Promise<void>
  userManager: UserManager | null
}

const AuthContext = createContext<AuthState | null>(null)

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [um, setUm] = useState<UserManager | null>(null)
  const [user, setUser] = useState<User | null>(null)
  const [isLoading, setIsLoading] = useState(true)
  const [loginError, setLoginError] = useState<string | null>(null)

  // Step 1: resolve UserManager (triggers the /config.json fetch once).
  useEffect(() => {
    getUserManager()
      .then(setUm)
      .catch(() => setIsLoading(false))
  }, [])

  // Step 2: subscribe to auth events once the manager is ready.
  useEffect(() => {
    if (!um) return

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
    um?.signinRedirect().catch((err: unknown) => {
      const msg = err instanceof Error ? err.message : String(err)
      setLoginError(
        msg.includes('Failed to fetch') || msg.includes('NetworkError') || msg.includes('CORS')
          ? 'Cannot reach the authentication server. Check your OIDC configuration and CORS settings.'
          : `Sign-in failed: ${msg}`,
      )
    })
  }, [um])
  const logout = useCallback(() => um?.signoutRedirect(), [um])
  const clearSession = useCallback(
    () => um?.removeUser() ?? Promise.resolve(),
    [um],
  )

  return (
    <AuthContext.Provider
      value={{
        user,
        isLoading: isLoading || !um,
        loginError,
        accessToken: user?.access_token,
        login,
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
