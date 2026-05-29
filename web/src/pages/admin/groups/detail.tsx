import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Link, useParams, useNavigate } from 'react-router-dom'
import { ArrowLeft, UserPlus, X } from 'lucide-react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Separator } from '@/components/ui/separator'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { DeleteButton } from '@/components/admin/delete-button'
import { useAuthClient } from '@/lib/api-client'
import { formatDate } from '@/lib/utils'
import { useAuth } from '@/auth/AuthContext'

export default function AdminGroupDetail() {
  const { slug } = useParams<{ slug: string }>()
  const { accessToken } = useAuth()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const api = useAuthClient()
  const [editOpen, setEditOpen] = useState(false)

  const { data: group, isPending, isError } = useQuery({
    queryKey: ['admin-group', slug],
    queryFn: () => api.GET('/api/v1/groups/{slug}', { params: { path: { slug: slug! } } }).then(r => r.data),
    enabled: !!slug && !!accessToken,
  })

  const { data: membersData } = useQuery({
    queryKey: ['admin-group-members', slug],
    queryFn: () => api.GET('/api/v1/groups/{slug}/members', { params: { path: { slug: slug! } } }).then(r => r.data),
    enabled: !!slug && !!accessToken,
  })

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ['admin-group', slug] })
    queryClient.invalidateQueries({ queryKey: ['admin-groups'] })
  }
  const invalidateMembers = () => queryClient.invalidateQueries({ queryKey: ['admin-group-members', slug] })

  const editMutation = useMutation({
    mutationFn: async (body: { name: string; description: string }) => {
      const { error } = await api.PATCH('/api/v1/groups/{slug}', { params: { path: { slug: slug! } }, body })
      if (error) throw new Error((error as { detail?: string; title?: string })?.detail ?? 'Update failed.')
    },
    onSuccess: () => { invalidate(); setEditOpen(false); toast.success('Group updated') },
    onError: (e: Error) => toast.error(e.message),
  })

  const deleteMutation = useMutation({
    mutationFn: async () => {
      const { error } = await api.DELETE('/api/v1/groups/{slug}', { params: { path: { slug: slug! } } })
      if (error) throw new Error((error as { title?: string }).title ?? 'Delete failed')
    },
    onSuccess: () => { queryClient.removeQueries({ queryKey: ['admin-groups'] }); navigate('/admin/groups') },
    onError: (e: Error) => toast.error(e.message),
  })

  const addMember = useMutation({
    mutationFn: async (email: string) => {
      const { error } = await api.PUT('/api/v1/groups/{slug}/members/{email}', {
        params: { path: { slug: slug!, email } },
      })
      if (error) throw new Error((error as { detail?: string; title?: string })?.detail ?? 'Failed to add member.')
    },
    onSuccess: () => { invalidateMembers(); toast.success('Member added') },
    onError: (e: Error) => toast.error(e.message),
  })

  const removeMember = useMutation({
    mutationFn: async (email: string) => {
      const { error } = await api.DELETE('/api/v1/groups/{slug}/members/{email}', {
        params: { path: { slug: slug!, email } },
      })
      if (error) throw new Error((error as { title?: string })?.title ?? 'Failed to remove member.')
    },
    onSuccess: () => { invalidateMembers(); toast.success('Member removed') },
    onError: (e: Error) => toast.error(e.message),
  })

  const members = membersData?.items ?? []

  if (isPending) return <p className="text-muted-foreground">Loading…</p>
  if (isError || !group) return (
    <div className="space-y-4">
      <p className="text-destructive">Not found.</p>
      <Button variant="outline" size="sm" onClick={() => navigate('/admin/groups')}>Back to Groups</Button>
    </div>
  )

  return (
    <div className="space-y-6 max-w-3xl mx-auto">
      <nav aria-label="Breadcrumb" className="flex items-center gap-1.5 text-sm text-muted-foreground">
        <Link to="/admin/groups" className="flex items-center gap-1 hover:text-foreground transition-colors">
          <ArrowLeft className="h-3.5 w-3.5" aria-hidden="true" />
          Groups
        </Link>
        <span aria-hidden="true">/</span>
        <span className="font-mono text-foreground">{group.slug}</span>
      </nav>

      <div>
        <h1 className="text-2xl font-bold">{group.name}</h1>
        <p className="text-sm text-muted-foreground font-mono mt-0.5">{group.slug}</p>
        {group.description && <p className="text-sm text-muted-foreground mt-2">{group.description}</p>}
      </div>

      <dl className="grid grid-cols-2 gap-x-4 gap-y-2 text-sm max-w-sm">
        <dt className="text-muted-foreground">Created</dt>
        <dd>{formatDate(group.created_at)}</dd>
        <dt className="text-muted-foreground">Updated</dt>
        <dd>{formatDate(group.updated_at)}</dd>
      </dl>

      {editOpen && (
        <form
          className="space-y-4 border rounded-lg p-4 max-w-sm"
          onSubmit={(e) => {
            e.preventDefault()
            const fd = new FormData(e.currentTarget)
            editMutation.mutate({ name: fd.get('name') as string, description: fd.get('description') as string })
          }}
        >
          <h2 className="text-lg font-semibold">Edit Group</h2>
          <div className="grid gap-3">
            <div className="space-y-1">
              <Label htmlFor="name">Name <span className="text-destructive">*</span></Label>
              <Input id="name" name="name" defaultValue={group.name} required />
            </div>
            <div className="space-y-1">
              <Label htmlFor="description">Description</Label>
              <Input id="description" name="description" defaultValue={group.description ?? ''} />
            </div>
          </div>
          <div className="flex gap-2">
            <Button type="submit" size="sm" disabled={editMutation.isPending}>
              {editMutation.isPending ? 'Saving…' : 'Save changes'}
            </Button>
            <Button type="button" variant="outline" size="sm" onClick={() => setEditOpen(false)}>Cancel</Button>
          </div>
        </form>
      )}

      <div className="flex flex-wrap gap-2">
        <Button variant="outline" size="sm" onClick={() => setEditOpen(v => !v)}>
          {editOpen ? 'Cancel edit' : 'Edit'}
        </Button>
        <DeleteButton onDelete={() => deleteMutation.mutate()} entityName={group.name} isPending={deleteMutation.isPending} />
      </div>

      <Separator />

      {/* Members */}
      <div className="space-y-3">
        <h2 className="text-lg font-semibold">Members <span className="text-sm font-normal text-muted-foreground">({members.length})</span></h2>
        <p className="text-sm text-muted-foreground">
          Adding an unknown email creates an invited user who can hold grants but
          cannot sign in until a credential is set. Federated users named in the
          OIDC <code>groups</code> claim are members implicitly and need not be listed here.
        </p>

        <form
          className="flex flex-wrap items-end gap-2"
          onSubmit={(e) => {
            e.preventDefault()
            const fd = new FormData(e.currentTarget)
            const email = (fd.get('email') as string).trim()
            if (email) {
              addMember.mutate(email)
              e.currentTarget.reset()
            }
          }}
        >
          <div className="space-y-1 flex-1 min-w-[12rem]">
            <Label htmlFor="member-email">Add member by email</Label>
            <Input id="member-email" name="email" type="email" placeholder="person@example.com" required />
          </div>
          <Button type="submit" size="sm" disabled={addMember.isPending} className="flex items-center gap-1.5">
            <UserPlus className="h-4 w-4" aria-hidden="true" /> Add
          </Button>
        </form>

        {members.length === 0 ? (
          <p className="text-sm text-muted-foreground py-2">No explicit members.</p>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Email</TableHead>
                <TableHead className="hidden sm:table-cell">Display name</TableHead>
                <TableHead />
              </TableRow>
            </TableHeader>
            <TableBody>
              {members.map((m) => (
                <TableRow key={m.id}>
                  <TableCell className="font-mono text-sm">{m.email}</TableCell>
                  <TableCell className="hidden sm:table-cell text-muted-foreground">{m.display_name || '—'}</TableCell>
                  <TableCell className="text-right">
                    <Button
                      variant="ghost"
                      size="sm"
                      aria-label={`Remove ${m.email}`}
                      disabled={removeMember.isPending}
                      onClick={() => removeMember.mutate(m.email)}
                    >
                      <X className="h-4 w-4" aria-hidden="true" />
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </div>
    </div>
  )
}
