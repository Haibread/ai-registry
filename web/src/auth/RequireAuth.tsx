import { Navigate, useLocation } from 'react-router-dom'
import { useAuth } from '@/auth/AuthContext'

interface Props { children: React.ReactNode }

// RequireAuth gates the /admin surface. Auth state comes from the session
// (AuthContext's GET /api/v1/me), not a JS token.
export function RequireAuth({ children }: Props) {
  const { isAuthenticated, authLoading } = useAuth()
  const location = useLocation()

  if (authLoading) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <p className="text-sm text-muted-foreground animate-pulse">Loading…</p>
      </div>
    )
  }

  // Not signed in (no session, or it lapsed after a 401) → send to the login
  // page, carrying the original destination so signing in lands back here
  // instead of losing the deep link.
  if (!isAuthenticated) {
    return (
      <Navigate
        to="/login"
        replace
        state={{ returnTo: location.pathname + location.search }}
      />
    )
  }

  return <>{children}</>
}
