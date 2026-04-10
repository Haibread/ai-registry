import { useEffect, useRef } from 'react'
import { useNavigate } from 'react-router-dom'
import { getUserManager } from './AuthContext'

export function AuthCallback() {
  const navigate = useNavigate()
  const called = useRef(false)

  useEffect(() => {
    if (called.current) return
    called.current = true

    getUserManager()
      .then((um) => um.signinRedirectCallback())
      .then(() => navigate('/admin', { replace: true }))
      .catch(() => navigate('/', { replace: true }))
  }, [navigate])

  return (
    <div className="flex min-h-screen items-center justify-center">
      <p className="text-sm text-muted-foreground animate-pulse">Signing you in…</p>
    </div>
  )
}
