import { Navigate } from 'react-router-dom'
import { useAuth } from './AuthContext'

interface Props { children: React.ReactNode }

export function RequireAuth({ children }: Props) {
  const { accessToken, isLoading } = useAuth()

  if (isLoading) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <p className="text-sm text-muted-foreground animate-pulse">Loading…</p>
      </div>
    )
  }

  // No token (never logged in, or session cleared after 401) → send to home.
  // The header's Sign In button lets the user initiate login intentionally.
  if (!accessToken) {
    return <Navigate to="/" replace />
  }

  return <>{children}</>
}
