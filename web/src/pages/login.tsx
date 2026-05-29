import { useState } from 'react'
import { Navigate, useNavigate, Link } from 'react-router-dom'
import { AlertCircle, LogIn } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { useAuth } from '@/auth/AuthContext'

export default function LoginPage() {
  const { accessToken, isLoading, login, loginLocal } = useAuth()
  const navigate = useNavigate()
  const [errorMsg, setErrorMsg] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  // Already signed in → straight to the admin surface.
  if (!isLoading && accessToken) {
    return <Navigate to="/admin" replace />
  }

  async function handleLocal(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault()
    setErrorMsg(null)
    const fd = new FormData(e.currentTarget)
    setSubmitting(true)
    try {
      await loginLocal((fd.get('email') as string).trim(), fd.get('password') as string)
      navigate('/admin')
    } catch (err) {
      setErrorMsg(err instanceof Error ? err.message : 'Sign-in failed.')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="flex min-h-screen items-center justify-center p-4">
      <div className="w-full max-w-sm space-y-6">
        <div className="text-center space-y-1">
          <Link to="/" className="inline-flex items-center gap-2 font-semibold">
            <span className="flex h-7 w-7 items-center justify-center rounded bg-primary text-primary-foreground text-xs font-bold">AI</span>
            <span>Registry</span>
          </Link>
          <h1 className="text-xl font-bold pt-2">Sign in</h1>
        </div>

        {errorMsg && (
          <div role="alert" className="flex items-start gap-2 rounded-md border border-destructive/50 bg-destructive/10 px-4 py-3 text-sm text-destructive">
            <AlertCircle className="h-4 w-4 mt-0.5 shrink-0" aria-hidden="true" />
            <p>{errorMsg}</p>
          </div>
        )}

        <Button type="button" className="w-full" onClick={login}>
          <LogIn className="h-4 w-4" aria-hidden="true" /> Sign in with your organization
        </Button>

        <div className="flex items-center gap-3 text-xs text-muted-foreground">
          <span className="h-px flex-1 bg-border" />
          or
          <span className="h-px flex-1 bg-border" />
        </div>

        <Card>
          <CardHeader>
            <CardTitle className="text-base">Email &amp; password</CardTitle>
            <CardDescription>For local accounts (no identity provider required).</CardDescription>
          </CardHeader>
          <CardContent>
            <form className="space-y-4" onSubmit={handleLocal}>
              <div className="space-y-1.5">
                <Label htmlFor="email">Email</Label>
                <Input id="email" name="email" type="email" autoComplete="username" required aria-required="true" />
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="password">Password</Label>
                <Input id="password" name="password" type="password" autoComplete="current-password" required aria-required="true" />
              </div>
              <Button type="submit" variant="outline" className="w-full" disabled={submitting}>
                {submitting ? 'Signing in…' : 'Sign in'}
              </Button>
            </form>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
