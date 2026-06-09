import { useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { useMutation } from '@tanstack/react-query'
import { toast } from 'sonner'
import { ArrowLeft, AlertCircle } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { useAuthClient } from '@/lib/api-client'

export default function AdminUserNew() {
  const api = useAuthClient()
  const navigate = useNavigate()
  const [errorMsg, setErrorMsg] = useState<string | null>(null)

  const mutation = useMutation({
    mutationFn: async (formData: FormData) => {
      const email = (formData.get('email') as string).trim()
      if (!email) throw new Error('Email is required.')
      const password = (formData.get('password') as string) || ''
      if (password !== '' && password.length < 8) {
        throw new Error('Password must be at least 8 characters (or leave it blank to invite without one).')
      }
      const { error } = await api.POST('/api/v1/users', {
        body: {
          email,
          display_name: (formData.get('display_name') as string) || undefined,
          password: password || undefined,
          is_server_admin: formData.get('is_server_admin') === 'on',
        },
      })
      if (error) {
        const msg = (error as { title?: string } | undefined)?.title
        throw new Error(msg ?? 'Failed to create user. The email may already be in use.')
      }
    },
    onSuccess: () => { toast.success('User created'); navigate('/admin/users') },
    onError: (err: Error) => setErrorMsg(err.message),
  })

  function handleSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault()
    setErrorMsg(null)
    mutation.mutate(new FormData(e.currentTarget))
  }

  return (
    <div className="space-y-6 max-w-lg mx-auto">
      <nav aria-label="Breadcrumb" className="flex items-center gap-1.5 text-sm text-muted-foreground">
        <Link to="/admin/users" className="flex items-center gap-1 hover:text-foreground transition-colors">
          <ArrowLeft className="h-3.5 w-3.5" aria-hidden="true" />
          Users
        </Link>
        <span aria-hidden="true">/</span>
        <span className="text-foreground">New User</span>
      </nav>

      <h1 className="text-2xl font-bold">New User</h1>

      {errorMsg && (
        <div role="alert" className="flex items-start gap-2 rounded-md border border-destructive/50 bg-destructive/10 px-4 py-3 text-sm text-destructive">
          <AlertCircle className="h-4 w-4 mt-0.5 shrink-0" aria-hidden="true" />
          <p>{errorMsg}</p>
        </div>
      )}

      <form onSubmit={handleSubmit}>
        <Card>
          <CardHeader>
            <CardTitle className="text-base">User Details</CardTitle>
            <CardDescription>
              Leave the password blank to create an invited user (can hold grants
              and memberships, but cannot sign in until a credential is set).
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-1.5">
              <Label htmlFor="email">
                Email <span className="text-destructive" aria-hidden="true">*</span>
              </Label>
              <Input id="email" name="email" type="email" placeholder="person@example.com" required aria-required="true" />
            </div>

            <div className="space-y-1.5">
              <Label htmlFor="display_name">Display name</Label>
              <Input id="display_name" name="display_name" placeholder="Ada Lovelace" />
            </div>

            <div className="space-y-1.5">
              <Label htmlFor="password">Password</Label>
              <Input id="password" name="password" type="password" autoComplete="new-password" placeholder="(optional — invite without one)" />
              <p className="text-xs text-muted-foreground">At least 8 characters, or leave blank.</p>
            </div>

            <label className="flex items-center gap-2 text-sm">
              <input type="checkbox" name="is_server_admin" className="h-4 w-4 rounded border-input" />
              Grant Server Admin (full access to all publishers)
            </label>
          </CardContent>
        </Card>

        <Button type="submit" className="w-full mt-6" disabled={mutation.isPending}>
          {mutation.isPending ? 'Creating…' : 'Create User'}
        </Button>
      </form>
    </div>
  )
}
