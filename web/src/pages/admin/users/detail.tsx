import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Link, useParams, useNavigate } from 'react-router-dom'
import { ArrowLeft, ShieldCheck } from 'lucide-react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Separator } from '@/components/ui/separator'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { useAuthClient } from '@/lib/api-client'
import { formatDate } from '@/lib/utils'

type UserPatch = {
  disabled?: boolean
  is_server_admin?: boolean
}

export default function AdminUserDetail() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const api = useAuthClient()

  const { data: user, isPending, isError } = useQuery({
    queryKey: ['admin-user', id],
    queryFn: () => api.GET('/api/v1/users/{id}', { params: { path: { id: id! } } }).then(r => r.data),
    enabled: !!id && true,
  })

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ['admin-user', id] })
    queryClient.invalidateQueries({ queryKey: ['admin-users'] })
  }

  const patch = useMutation({
    mutationFn: async (body: UserPatch) => {
      const { error } = await api.PATCH('/api/v1/users/{id}', { params: { path: { id: id! } }, body })
      if (error) throw new Error((error as { detail?: string; title?: string })?.detail ?? 'Update failed.')
    },
    onSuccess: () => { invalidate(); toast.success('User updated') },
    onError: (e: Error) => toast.error(e.message),
  })

  const setPassword = useMutation({
    mutationFn: async (password: string) => {
      const { error } = await api.POST('/api/v1/users/{id}/set-password', {
        params: { path: { id: id! } },
        body: { password },
      })
      if (error) throw new Error((error as { detail?: string; title?: string })?.detail ?? 'Failed to set password.')
    },
    onSuccess: () => toast.success('Password set'),
    onError: (e: Error) => toast.error(e.message),
  })

  if (isPending) return <p className="text-muted-foreground">Loading…</p>
  if (isError || !user) return (
    <div className="space-y-4">
      <p className="text-destructive">Not found.</p>
      <Button variant="outline" size="sm" onClick={() => navigate('/admin/users')}>Back to Users</Button>
    </div>
  )

  return (
    <div className="space-y-6 max-w-2xl mx-auto">
      <nav aria-label="Breadcrumb" className="flex items-center gap-1.5 text-sm text-muted-foreground">
        <Link to="/admin/users" className="flex items-center gap-1 hover:text-foreground transition-colors">
          <ArrowLeft className="h-3.5 w-3.5" aria-hidden="true" />
          Users
        </Link>
        <span aria-hidden="true">/</span>
        <span className="font-mono text-foreground">{user.email}</span>
      </nav>

      <div className="flex items-start gap-3 flex-wrap">
        <div className="flex-1">
          <h1 className="text-2xl font-bold">{user.display_name || user.email}</h1>
          <p className="text-sm text-muted-foreground font-mono mt-0.5">{user.email}</p>
        </div>
        {user.is_server_admin && (
          <Badge variant="success" className="gap-1">
            <ShieldCheck className="h-3 w-3" aria-hidden="true" /> Server Admin
          </Badge>
        )}
        {user.disabled && <Badge variant="muted">Disabled</Badge>}
      </div>

      <dl className="grid grid-cols-2 gap-x-4 gap-y-2 text-sm max-w-sm">
        <dt className="text-muted-foreground">Federated</dt>
        <dd>{user.subject ? 'yes' : 'no'}</dd>
        <dt className="text-muted-foreground">Local password</dt>
        <dd>{user.has_password ? 'set' : 'not set'}</dd>
        <dt className="text-muted-foreground">Last seen</dt>
        <dd>{user.last_seen_at ? formatDate(user.last_seen_at) : 'never'}</dd>
        <dt className="text-muted-foreground">Created</dt>
        <dd>{formatDate(user.created_at)}</dd>
      </dl>

      <Separator />

      <div className="space-y-3">
        <h2 className="text-lg font-semibold">Actions</h2>
        <div className="flex flex-wrap gap-2">
          <Button
            variant="outline"
            size="sm"
            disabled={patch.isPending}
            onClick={() => patch.mutate({ disabled: !user.disabled })}
          >
            {user.disabled ? 'Enable account' : 'Disable account'}
          </Button>
          <Button
            variant="outline"
            size="sm"
            disabled={patch.isPending}
            onClick={() => patch.mutate({ is_server_admin: !user.is_server_admin })}
          >
            {user.is_server_admin ? 'Revoke Server Admin' : 'Grant Server Admin'}
          </Button>
        </div>
      </div>

      <Separator />

      <form
        className="space-y-3 max-w-sm"
        onSubmit={(e) => {
          e.preventDefault()
          const fd = new FormData(e.currentTarget)
          const pw = fd.get('password') as string
          if (pw) {
            setPassword.mutate(pw)
            e.currentTarget.reset()
          }
        }}
      >
        <h2 className="text-lg font-semibold">Set password</h2>
        <p className="text-sm text-muted-foreground">
          Sets a local password so this user can sign in with email + password.
        </p>
        <div className="space-y-1">
          <Label htmlFor="password">New password</Label>
          <Input id="password" name="password" type="password" minLength={8} autoComplete="new-password" required />
        </div>
        <Button type="submit" size="sm" disabled={setPassword.isPending}>
          {setPassword.isPending ? 'Setting…' : 'Set password'}
        </Button>
      </form>
    </div>
  )
}
