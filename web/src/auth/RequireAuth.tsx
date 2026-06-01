import { Navigate } from 'react-router-dom'
import { useAuth } from '@/auth/AuthContext'

interface Props { children: React.ReactNode }

// RequireAuth gates the /admin surface. Auth state comes from the session
// (AuthContext's GET /api/v1/me), not a JS token.
export function RequireAuth({ children }: Props) {
  const { isAuthenticated, authLoading } = useAuth()

  if (authLoading) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <p className="text-sm text-muted-foreground animate-pulse">Loading…</p>
      </div>
    )
  }

  // Not signed in (no session, or it lapsed after a 401) → send home. The
  // header's Sign in button lets the user initiate login intentionally.
  if (!isAuthenticated) {
    return <Navigate to="/" replace />
  }

  return <>{children}</>
}
