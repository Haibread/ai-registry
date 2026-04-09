import { createContext, useContext, useEffect, useRef, useState } from 'react'
import { UserManager, type User } from 'oidc-client-ts'

const userManager = new UserManager({
  authority: import.meta.env.VITE_OIDC_ISSUER ?? 'http://localhost:8080/realms/ai-registry',
  client_id: import.meta.env.VITE_OIDC_CLIENT_ID ?? 'ai-registry-web',
  redirect_uri: window.location.origin + '/auth/callback',
  post_logout_redirect_uri: window.location.origin,
  response_type: 'code',
  scope: 'openid profile email',
  automaticSilentRenew: true,
})

interface AuthState {
  user: User | null
  isLoading: boolean
  accessToken: string | undefined
  login: () => void
  logout: () => void
  clearSession: () => Promise<void>
  userManager: UserManager
}

const AuthContext = createContext<AuthState | null>(null)

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [user, setUser] = useState<User | null>(null)
  const [isLoading, setIsLoading] = useState(true)
  const initialized = useRef(false)

  useEffect(() => {
    if (initialized.current) return
    initialized.current = true

    userManager.getUser().then((u) => {
      setUser(u)
      setIsLoading(false)
    }).catch(() => setIsLoading(false))

    const onUserLoaded = (u: User) => setUser(u)
    const onUserUnloaded = () => setUser(null)
    userManager.events.addUserLoaded(onUserLoaded)
    userManager.events.addUserUnloaded(onUserUnloaded)

    return () => {
      userManager.events.removeUserLoaded(onUserLoaded)
      userManager.events.removeUserUnloaded(onUserUnloaded)
    }
  }, [])

  const login = () => userManager.signinRedirect()
  const logout = () => userManager.signoutRedirect()
  // Clears the local session without a Keycloak redirect — used when the
  // server returns 401 (expired or revoked token).
  const clearSession = () => userManager.removeUser()

  return (
    <AuthContext.Provider value={{
      user,
      isLoading,
      accessToken: user?.access_token,
      login,
      logout,
      clearSession,
      userManager,
    }}>
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth() {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used inside AuthProvider')
  return ctx
}

export { userManager }
