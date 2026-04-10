import { useEffect, useRef } from 'react'
import { getUserManager } from './AuthContext'

export function SessionExpired() {
  const called = useRef(false)
  useEffect(() => {
    if (called.current) return
    called.current = true
    getUserManager().then((um) => um.signoutRedirect())
  }, [])

  return (
    <div className="flex min-h-screen items-center justify-center">
      <p className="text-sm text-muted-foreground animate-pulse">
        Session expired — signing you out…
      </p>
    </div>
  )
}
