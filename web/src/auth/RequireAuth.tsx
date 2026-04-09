import { useEffect } from 'react'
import { useAuth } from './AuthContext'

interface Props { children: React.ReactNode }

export function RequireAuth({ children }: Props) {
  const { accessToken, isLoading, login } = useAuth()

  useEffect(() => {
    if (!isLoading && !accessToken) {
      login()
    }
  }, [isLoading, accessToken, login])

  if (isLoading) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <p className="text-sm text-muted-foreground animate-pulse">Loading…</p>
      </div>
    )
  }

  if (!accessToken) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <p className="text-sm text-muted-foreground animate-pulse">Redirecting to sign in…</p>
      </div>
    )
  }

  return <>{children}</>
}
