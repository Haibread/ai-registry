import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Briefcase, Plus, X } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { DeleteButton } from '@/components/admin/delete-button'
import { useAuthClient } from '@/lib/api-client'
import { useAuth } from '@/auth/AuthContext'
import { formatDate } from '@/lib/utils'
import type { components } from '@/lib/schema'

type Workspace = components['schemas']['Workspace']

interface WorkspacesSectionProps {
  publisherSlug: string
}

// 409-type → friendly message for the workspace mutations.
function friendlyError(error: unknown, fallback: string): string {
  const e = error as { type?: string; detail?: string }
  if (e?.type?.includes('conflict')) return e.detail || 'Slug already exists in this publisher.'
  return e?.detail || fallback
}

export function WorkspacesSection({ publisherSlug }: WorkspacesSectionProps) {
  const { accessToken } = useAuth()
  const api = useAuthClient()
  const queryClient = useQueryClient()

  const [createOpen, setCreateOpen] = useState(false)
  const [editingSlug, setEditingSlug] = useState<string | null>(null)
  const [actionError, setActionError] = useState<string | null>(null)

  const queryKey = ['admin-publisher-workspaces', publisherSlug]

  const { data, isPending, isError } = useQuery({
    queryKey,
    queryFn: () =>
      api
        .GET('/api/v1/publishers/{publisher_slug}/workspaces', {
          params: { path: { publisher_slug: publisherSlug }, query: { limit: 50 } },
        })
        .then((r) => r.data),
    enabled: !!accessToken,
  })

  const items: Workspace[] = (data?.items ?? []) as Workspace[]

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey })
  }

  const createMutation = useMutation({
    mutationFn: async (body: { slug: string; name: string; description?: string }) => {
      setActionError(null)
      const { error } = await api.POST('/api/v1/publishers/{publisher_slug}/workspaces', {
        params: { path: { publisher_slug: publisherSlug } },
        body,
      })
      if (error) throw new Error(friendlyError(error, 'Create failed.'))
    },
    onSuccess: () => {
      setCreateOpen(false)
      invalidate()
    },
    onError: (err: Error) => setActionError(err.message),
  })

  const patchMutation = useMutation({
    mutationFn: async ({
      workspaceSlug,
      body,
    }: {
      workspaceSlug: string
      body: { name?: string; description?: string; contact?: string; group_name?: string }
    }) => {
      setActionError(null)
      const { error } = await api.PATCH(
        '/api/v1/publishers/{publisher_slug}/workspaces/{workspace_slug}',
        {
          params: { path: { publisher_slug: publisherSlug, workspace_slug: workspaceSlug } },
          body,
        },
      )
      if (error) throw new Error(friendlyError(error, 'Update failed.'))
    },
    onSuccess: () => {
      setEditingSlug(null)
      invalidate()
    },
    onError: (err: Error) => setActionError(err.message),
  })

  const deleteMutation = useMutation({
    mutationFn: async (workspaceSlug: string) => {
      setActionError(null)
      const { error } = await api.DELETE(
        '/api/v1/publishers/{publisher_slug}/workspaces/{workspace_slug}',
        {
          params: { path: { publisher_slug: publisherSlug, workspace_slug: workspaceSlug } },
        },
      )
      if (error) {
        // 409 here means the workspace still owns resources.
        const e = error as { status?: number; detail?: string }
        const msg = e?.status === 409
          ? 'Cannot delete: workspace still has MCP servers or agents.'
          : e?.detail || 'Delete failed.'
        throw new Error(msg)
      }
    },
    onSuccess: invalidate,
    onError: (err: Error) => setActionError(err.message),
  })

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold flex items-center gap-2">
          <Briefcase className="h-4 w-4" aria-hidden="true" />
          Workspaces
          <span className="text-sm font-normal text-muted-foreground">({items.length})</span>
        </h2>
        <Button size="sm" onClick={() => { setCreateOpen((v) => !v); setActionError(null) }}>
          {createOpen ? <X className="h-4 w-4" /> : <Plus className="h-4 w-4" />}
          <span className="ml-1.5">{createOpen ? 'Cancel' : 'New workspace'}</span>
        </Button>
      </div>
      <p className="text-sm text-muted-foreground">
        Workspaces group MCP servers and agents under this publisher and bind
        each set to a Keycloak group whose members can author content.
      </p>

      {actionError && (
        <div role="alert" className="rounded border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {actionError}
        </div>
      )}

      {createOpen && (
        <form
          className="space-y-4 border rounded-lg p-4 max-w-xl"
          onSubmit={(e) => {
            e.preventDefault()
            const fd = new FormData(e.currentTarget)
            createMutation.mutate({
              slug: (fd.get('slug') as string).trim(),
              name: (fd.get('name') as string).trim(),
              description: ((fd.get('description') as string) || '').trim() || undefined,
            })
          }}
        >
          <h3 className="text-sm font-semibold">New workspace</h3>
          <div className="grid gap-3 md:grid-cols-2">
            <div className="space-y-1">
              <Label htmlFor="ws-slug">Slug <span className="text-destructive">*</span></Label>
              <Input id="ws-slug" name="slug" required pattern="[a-z0-9-]+" placeholder="claude-team" />
            </div>
            <div className="space-y-1">
              <Label htmlFor="ws-name">Name <span className="text-destructive">*</span></Label>
              <Input id="ws-name" name="name" required placeholder="Claude team" />
            </div>
          </div>
          <div className="space-y-1">
            <Label htmlFor="ws-desc">Description</Label>
            <Input id="ws-desc" name="description" placeholder="Stuff the Claude team owns" />
          </div>
          <div className="flex gap-2">
            <Button type="submit" size="sm" disabled={createMutation.isPending}>
              {createMutation.isPending ? 'Creating…' : 'Create workspace'}
            </Button>
            <Button type="button" variant="outline" size="sm" onClick={() => setCreateOpen(false)}>
              Cancel
            </Button>
          </div>
        </form>
      )}

      {isPending ? (
        <p className="text-sm text-muted-foreground py-4">Loading workspaces…</p>
      ) : isError ? (
        <p className="text-sm text-destructive py-4">Failed to load workspaces.</p>
      ) : items.length === 0 ? (
        <p className="text-sm text-muted-foreground py-4">
          No workspaces yet. Create one to start delegating writes via Keycloak groups.
        </p>
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Slug</TableHead>
              <TableHead>Name</TableHead>
              <TableHead>Group</TableHead>
              <TableHead>Updated</TableHead>
              <TableHead />
            </TableRow>
          </TableHeader>
          <TableBody>
            {items.map((ws) => {
              const isEditing = editingSlug === ws.slug
              return (
                <TableRow key={ws.id}>
                  <TableCell className="font-mono text-sm">{ws.slug}</TableCell>
                  <TableCell className="font-medium">{ws.name}</TableCell>
                  <TableCell>
                    {ws.group_name ? (
                      <Badge variant="secondary" className="font-mono text-xs">
                        {ws.group_name}
                      </Badge>
                    ) : (
                      <span className="text-xs text-muted-foreground">admin-only</span>
                    )}
                  </TableCell>
                  <TableCell className="text-muted-foreground">{formatDate(ws.updated_at)}</TableCell>
                  <TableCell className="text-right">
                    <div className="inline-flex gap-1">
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => {
                          setEditingSlug(isEditing ? null : ws.slug)
                          setActionError(null)
                        }}
                      >
                        {isEditing ? 'Cancel' : 'Edit'}
                      </Button>
                      <DeleteButton
                        onDelete={() => deleteMutation.mutate(ws.slug)}
                        entityName={ws.name}
                        isPending={
                          deleteMutation.isPending &&
                          deleteMutation.variables === ws.slug
                        }
                      />
                    </div>
                  </TableCell>
                </TableRow>
              )
            })}
          </TableBody>
        </Table>
      )}

      {/* Edit form: rendered below the table, scoped to the editing row. */}
      {editingSlug && (
        <EditWorkspaceForm
          workspace={items.find((w) => w.slug === editingSlug)!}
          onSubmit={(body) =>
            patchMutation.mutate({ workspaceSlug: editingSlug, body })
          }
          onCancel={() => setEditingSlug(null)}
          isPending={patchMutation.isPending}
        />
      )}
    </div>
  )
}

function EditWorkspaceForm({
  workspace,
  onSubmit,
  onCancel,
  isPending,
}: {
  workspace: Workspace
  onSubmit: (body: { name: string; description?: string; contact?: string; group_name: string }) => void
  onCancel: () => void
  isPending: boolean
}) {
  return (
    <form
      className="space-y-4 border rounded-lg p-4 max-w-xl"
      onSubmit={(e) => {
        e.preventDefault()
        const fd = new FormData(e.currentTarget)
        onSubmit({
          name: (fd.get('name') as string).trim(),
          description: ((fd.get('description') as string) || '').trim() || undefined,
          contact: ((fd.get('contact') as string) || '').trim() || undefined,
          // Empty string clears the binding back to admin-only on the server.
          group_name: (fd.get('group_name') as string).trim(),
        })
      }}
    >
      <h3 className="text-sm font-semibold">
        Edit workspace <span className="font-mono">{workspace.slug}</span>
      </h3>
      <div className="grid gap-3 md:grid-cols-2">
        <div className="space-y-1">
          <Label htmlFor="edit-name">Name <span className="text-destructive">*</span></Label>
          <Input id="edit-name" name="name" required defaultValue={workspace.name} />
        </div>
        <div className="space-y-1">
          <Label htmlFor="edit-group">Keycloak group</Label>
          <Input
            id="edit-group"
            name="group_name"
            defaultValue={workspace.group_name ?? ''}
            placeholder="claude-team (leave empty for admin-only)"
          />
        </div>
      </div>
      <div className="space-y-1">
        <Label htmlFor="edit-desc">Description</Label>
        <Input id="edit-desc" name="description" defaultValue={workspace.description ?? ''} />
      </div>
      <div className="space-y-1">
        <Label htmlFor="edit-contact">Contact</Label>
        <Input id="edit-contact" name="contact" defaultValue={workspace.contact ?? ''} />
      </div>
      <p className="text-xs text-muted-foreground">
        The Keycloak group whose members can author this workspace's content. Empty
        clears the binding so only admins can write.
      </p>
      <div className="flex gap-2">
        <Button type="submit" size="sm" disabled={isPending}>
          {isPending ? 'Saving…' : 'Save changes'}
        </Button>
        <Button type="button" variant="outline" size="sm" onClick={onCancel}>
          Cancel
        </Button>
      </div>
    </form>
  )
}
