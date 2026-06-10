import { useState } from 'react'
import { Link } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { X, Plus } from 'lucide-react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { ConfirmDialog } from '@/components/ui/confirm-dialog'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import { NativeSelect } from '@/components/ui/native-select'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { useAuthClient } from '@/lib/api-client'
import { usePermissions } from '@/auth/useMe'

type PrincipalType = 'user' | 'group'
type Role = 'viewer' | 'editor' | 'reviewer' | 'admin'
const ROLES: Role[] = ['viewer', 'editor', 'reviewer', 'admin']

interface GrantsSectionProps {
  // When set, manages that publisher's grants; otherwise global (all-publishers).
  publisherSlug?: string
}

/**
 * GrantsSection lists and manages role grants for one scope — a single
 * publisher (publisherSlug set) or the global all-publishers scope. It is the
 * shared UI behind the publisher detail page and the global Grants page.
 */
export function GrantsSection({ publisherSlug }: GrantsSectionProps) {
  const api = useAuthClient()
  const queryClient = useQueryClient()
  const scopeKey = publisherSlug ?? 'global'

  // Group grants bind an IdP claim to a role, so the server reserves them for
  // Server Admins; publisher Admins manage only per-user grants. Gate the UI to
  // match — the server enforces the same rule regardless.
  const { isServerAdmin } = usePermissions()
  const canManageGroups = isServerAdmin

  const [principalTypeState, setPrincipalType] = useState<PrincipalType>('group')
  const principalType: PrincipalType = canManageGroups ? principalTypeState : 'user'
  const [principalId, setPrincipalId] = useState('')
  const [role, setRole] = useState<Role>('editor')
  // Grant whose revocation awaits confirmation — this is the RBAC surface, so
  // a single stray X-click must not remove access (P1.4).
  const [revokeTarget, setRevokeTarget] = useState<{ id: string; role: string; label: string } | null>(null)

  const grantsQuery = useQuery({
    queryKey: ['admin-grants', scopeKey],
    queryFn: async () => {
      if (publisherSlug) {
        const r = await api.GET('/api/v1/publishers/{slug}/grants', { params: { path: { slug: publisherSlug } } })
        return r.data
      }
      const r = await api.GET('/api/v1/grants')
      return r.data
    },
    enabled: true,
  })

  const groupsQuery = useQuery({
    queryKey: ['admin-groups'],
    queryFn: () => api.GET('/api/v1/groups', { params: { query: { limit: 100 } } }).then(r => r.data),
    enabled: true,
  })
  const usersQuery = useQuery({
    queryKey: ['admin-users'],
    queryFn: () => api.GET('/api/v1/users', { params: { query: { limit: 100 } } }).then(r => r.data),
    enabled: true,
  })

  const invalidate = () => queryClient.invalidateQueries({ queryKey: ['admin-grants', scopeKey] })

  const addGrant = useMutation({
    mutationFn: async () => {
      if (!principalId) throw new Error('Pick a principal first.')
      const body = { principal_type: principalType, principal_id: principalId, role }
      const { error } = publisherSlug
        ? await api.POST('/api/v1/publishers/{slug}/grants', { params: { path: { slug: publisherSlug } }, body })
        : await api.POST('/api/v1/grants', { body })
      if (error) throw new Error((error as { detail?: string; title?: string })?.detail ?? 'Failed to create grant.')
    },
    onSuccess: () => { invalidate(); setPrincipalId(''); toast.success('Grant added') },
    onError: (e: Error) => toast.error(e.message),
  })

  const deleteGrant = useMutation({
    mutationFn: async (id: string) => {
      const { error } = publisherSlug
        ? await api.DELETE('/api/v1/publishers/{slug}/grants/{id}', { params: { path: { slug: publisherSlug, id } } })
        : await api.DELETE('/api/v1/grants/{id}', { params: { path: { id } } })
      if (error) throw new Error((error as { title?: string })?.title ?? 'Failed to revoke grant.')
    },
    onSuccess: () => { invalidate(); toast.success('Grant revoked') },
    onError: (e: Error) => toast.error(e.message),
  })

  const grants = grantsQuery.data?.items ?? []
  const principals = principalType === 'group'
    ? (groupsQuery.data?.items ?? []).map(g => ({ id: g.id, label: g.slug }))
    : (usersQuery.data?.items ?? []).map(u => ({ id: u.id, label: u.email }))

  return (
    <div className="space-y-3">
      <h2 className="text-lg font-semibold">Role grants</h2>
      <p className="text-sm text-muted-foreground max-w-prose">
        {publisherSlug
          ? 'Roles granted on this publisher. Editor authors, Reviewer approves, Admin manages; Viewer reads private entries.'
          : 'Global grants apply on every publisher. Use sparingly — prefer per-publisher grants.'}{' '}
        <Link to="/admin/help/roles" className="text-primary hover:underline">
          What can each role do?
        </Link>
      </p>

      <form
        className="flex flex-wrap items-end gap-2"
        onSubmit={(e) => { e.preventDefault(); addGrant.mutate() }}
      >
        {/* Labels are block-level so they stack above their selects instead
            of collapsing inline ("Principal[Group ▾]"). */}
        <div className="space-y-1">
          <Label htmlFor="principal-type" className="block">Principal</Label>
          <NativeSelect
            id="principal-type"
            value={principalType}
            disabled={!canManageGroups}
            onChange={(e) => { setPrincipalType(e.target.value as PrincipalType); setPrincipalId('') }}
          >
            {canManageGroups && <option value="group">Group</option>}
            <option value="user">User</option>
          </NativeSelect>
        </div>

        <div className="space-y-1 min-w-[12rem]">
          <Label htmlFor="principal-id" className="block">{principalType === 'group' ? 'Group' : 'User'}</Label>
          <NativeSelect
            id="principal-id"
            className="w-full"
            value={principalId}
            onChange={(e) => setPrincipalId(e.target.value)}
          >
            <option value="">Select…</option>
            {principals.map(p => (
              <option key={p.id} value={p.id}>{p.label}</option>
            ))}
          </NativeSelect>
        </div>

        <div className="space-y-1">
          <Label htmlFor="grant-role" className="block">Role</Label>
          <NativeSelect id="grant-role" value={role} onChange={(e) => setRole(e.target.value as Role)}>
            {ROLES.map(r => <option key={r} value={r}>{r}</option>)}
          </NativeSelect>
        </div>

        {/* Enabled even with no principal picked: submitting then surfaces
            "Pick a principal first" as a toast, which keyboard and touch
            users can actually discover (a disabled button explains nothing). */}
        <Button type="submit" size="sm" disabled={addGrant.isPending} className="flex items-center gap-1.5">
          <Plus className="h-4 w-4" aria-hidden="true" /> Grant
        </Button>
      </form>

      {!canManageGroups && (
        <p className="text-sm text-muted-foreground">
          Group grants are managed by your instance admin.
        </p>
      )}

      {grants.length === 0 ? (
        <p className="text-sm text-muted-foreground py-2">No grants{publisherSlug ? ' on this publisher' : ''} yet.</p>
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Principal</TableHead>
              <TableHead>Type</TableHead>
              <TableHead>Role</TableHead>
              <TableHead />
            </TableRow>
          </TableHeader>
          <TableBody>
            {grants.map((g) => (
              <TableRow key={g.id}>
                <TableCell className="font-mono text-sm">{g.principal_label || g.principal_id}</TableCell>
                <TableCell className="text-muted-foreground">{g.principal_type}</TableCell>
                <TableCell>
                  <Badge variant="secondary">{g.role}</Badge>
                  {g.source === 'config' && (
                    <Badge
                      variant="muted"
                      className="ml-1"
                      title="Seeded from the server's bootstrap file; re-applied on every boot"
                    >
                      config
                    </Badge>
                  )}
                </TableCell>
                <TableCell className="text-right">
                  {g.source === 'config' ? (
                    // Revoking a config grant would not stick — bootstrap
                    // re-seeds it on the next server start. Say so instead
                    // of offering a revoke that silently comes back.
                    <span className="text-xs text-muted-foreground">
                      remove from bootstrap file
                    </span>
                  ) : (
                    (g.principal_type !== 'group' || canManageGroups) && (
                      <Button
                        variant="ghost"
                        size="sm"
                        aria-label={`Revoke ${g.role} from ${g.principal_label || g.principal_id}`}
                        disabled={deleteGrant.isPending}
                        onClick={() =>
                          setRevokeTarget({ id: g.id, role: g.role, label: g.principal_label || g.principal_id })
                        }
                      >
                        <X className="h-4 w-4" aria-hidden="true" />
                      </Button>
                    )
                  )}
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}

      <ConfirmDialog
        open={revokeTarget !== null}
        onOpenChange={(o) => { if (!o) setRevokeTarget(null) }}
        title={revokeTarget ? `Revoke ${revokeTarget.role} from "${revokeTarget.label}"?` : ''}
        description={
          publisherSlug
            ? `The principal immediately loses its ${revokeTarget?.role ?? ''} access on ${publisherSlug}.`
            : `The principal immediately loses its global ${revokeTarget?.role ?? ''} access (all publishers).`
        }
        confirmLabel="Revoke grant"
        destructive
        isPending={deleteGrant.isPending}
        onConfirm={() => {
          if (revokeTarget) deleteGrant.mutate(revokeTarget.id)
          setRevokeTarget(null)
        }}
      />
    </div>
  )
}
