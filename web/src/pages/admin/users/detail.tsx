import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Link, useParams, useNavigate } from 'react-router-dom'
import { ArrowLeft, ShieldCheck } from 'lucide-react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { ConfirmDialog } from '@/components/ui/confirm-dialog'
import { Separator } from '@/components/ui/separator'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { DetailPageSkeleton } from '@/components/ui/detail-page-skeleton'
import { ErrorState } from '@/components/ui/error-state'
import { useAuthClient } from '@/lib/api-client'
import { useMe } from '@/auth/useMe'
import { formatDate, problemMessage, HTTPError, isNotFound } from '@/lib/utils'

type UserPatch = {
  display_name?: string
  disabled?: boolean
  is_server_admin?: boolean
}

// Which consequential action is awaiting confirmation.
type PendingAction = 'disable' | 'admin' | null

export default function AdminUserDetail() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const api = useAuthClient()
  const { data: me } = useMe()
  const [pendingAction, setPendingAction] = useState<PendingAction>(null)

  const { data: user, isPending, isError, error, refetch } = useQuery({
    queryKey: ['admin-user', id],
    queryFn: async () => {
      const { data, error, response } = await api.GET('/api/v1/users/{id}', { params: { path: { id: id! } } })
      // Carry the HTTP status so the error branch can tell a real 404 from
      // a server error or network failure (P2.6).
      if (error || !data) throw new HTTPError(problemMessage(error, 'Failed to load this user.'), response?.status)
      return data
    },
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
    // Refresh the user so the "Local password" row reflects the new state.
    onSuccess: () => { invalidate(); toast.success('Password set') },
    onError: (e: Error) => toast.error(e.message),
  })

  const isSelf = !!me?.user_id && me.user_id === id

  if (isPending) return <DetailPageSkeleton />
  if (isError || !user) return (
    <div className="space-y-4">
      {isNotFound(error) ? (
        <p className="text-destructive">Not found — this user may have been removed.</p>
      ) : (
        <ErrorState
          message={error instanceof Error ? error.message : 'Failed to load this user.'}
          onRetry={() => refetch()}
        />
      )}
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
          {/* Only show the email subtitle when the heading is a display
              name — otherwise the email would print twice. */}
          {user.display_name && (
            <p className="text-sm text-muted-foreground font-mono mt-0.5">{user.email}</p>
          )}
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
        {/* Both actions reshape who can sign in / administer the registry, so
            they confirm first (P1.4). Pointing them at yourself is blocked —
            the server enforces the same rule (lockout protection). */}
        <div className="flex flex-wrap gap-2">
          <Button
            variant="outline"
            size="sm"
            disabled={patch.isPending || (isSelf && !user.disabled)}
            title={isSelf && !user.disabled ? 'You cannot disable your own account' : undefined}
            onClick={() => setPendingAction('disable')}
          >
            {user.disabled ? 'Enable account' : 'Disable account'}
          </Button>
          <Button
            variant="outline"
            size="sm"
            disabled={patch.isPending || (isSelf && user.is_server_admin)}
            title={isSelf && user.is_server_admin ? 'You cannot revoke your own Server Admin role' : undefined}
            onClick={() => setPendingAction('admin')}
          >
            {user.is_server_admin ? 'Revoke Server Admin' : 'Grant Server Admin'}
          </Button>
        </div>
      </div>

      <ConfirmDialog
        open={pendingAction === 'disable'}
        onOpenChange={(o) => { if (!o) setPendingAction(null) }}
        title={user.disabled ? `Enable account "${user.email}"?` : `Disable account "${user.email}"?`}
        description={
          user.disabled
            ? 'The user will be able to sign in again.'
            : 'The user will no longer be able to sign in. Existing sessions stop working at the next token refresh.'
        }
        confirmLabel={user.disabled ? 'Enable account' : 'Disable account'}
        destructive={!user.disabled}
        isPending={patch.isPending}
        onConfirm={() => {
          patch.mutate({ disabled: !user.disabled })
          setPendingAction(null)
        }}
      />
      <ConfirmDialog
        open={pendingAction === 'admin'}
        onOpenChange={(o) => { if (!o) setPendingAction(null) }}
        title={
          user.is_server_admin
            ? `Revoke Server Admin from "${user.email}"?`
            : `Grant Server Admin to "${user.email}"?`
        }
        description={
          user.is_server_admin
            ? 'The user loses access to every publisher and all server administration.'
            : 'Server Admins bypass publisher roles and the review queue, and can manage every user, publisher, and entry.'
        }
        confirmLabel={user.is_server_admin ? 'Revoke Server Admin' : 'Grant Server Admin'}
        destructive={user.is_server_admin}
        isPending={patch.isPending}
        onConfirm={() => {
          patch.mutate({ is_server_admin: !user.is_server_admin })
          setPendingAction(null)
        }}
      />

      <Separator />

      <form
        className="space-y-3 max-w-sm"
        onSubmit={(e) => {
          e.preventDefault()
          const fd = new FormData(e.currentTarget)
          patch.mutate({ display_name: (fd.get('display_name') as string).trim() })
        }}
      >
        <h2 className="text-lg font-semibold">Display name</h2>
        <p className="text-sm text-muted-foreground max-w-prose">
          Shown instead of the email across the admin console.
        </p>
        <div className="space-y-1">
          <Label htmlFor="display_name">Display name</Label>
          <Input
            id="display_name"
            name="display_name"
            defaultValue={user.display_name ?? ''}
            placeholder="Jane Doe"
            autoComplete="off"
          />
        </div>
        <Button type="submit" size="sm" variant="outline" disabled={patch.isPending}>
          {patch.isPending ? 'Saving…' : 'Save display name'}
        </Button>
      </form>

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
