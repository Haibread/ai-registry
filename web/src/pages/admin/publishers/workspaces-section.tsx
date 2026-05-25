import { Fragment, useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { Briefcase, Plus, X, ChevronRight, ChevronDown, Server, Bot, Pencil, ArrowRight } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { TableSkeleton } from '@/components/ui/table-skeleton'
import { StatusBadge } from '@/components/ui/badge'
import { DeleteButton } from '@/components/admin/delete-button'
import { useAuthClient } from '@/lib/api-client'
import { useAuth } from '@/auth/AuthContext'
import { formatDate } from '@/lib/utils'
import type { components } from '@/lib/schema'

type Workspace = components['schemas']['Workspace']
type MCPServer = components['schemas']['MCPServer']
type Agent = components['schemas']['Agent']

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
  const [expandedSlugs, setExpandedSlugs] = useState<Set<string>>(new Set())
  // Errors are tracked per source so they render next to the action that
  // produced them (inline form errors) rather than as a single global
  // banner at the top of the section.
  const [createError, setCreateError] = useState<string | null>(null)
  const [editError, setEditError] = useState<string | null>(null)
  const [deleteError, setDeleteError] = useState<string | null>(null)

  const toggleExpanded = (workspaceSlug: string) => {
    setExpandedSlugs((prev) => {
      const next = new Set(prev)
      if (next.has(workspaceSlug)) next.delete(workspaceSlug)
      else next.add(workspaceSlug)
      return next
    })
  }

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
    mutationFn: async (body: { slug: string; name: string; description?: string; group_name?: string }) => {
      setCreateError(null)
      const { error } = await api.POST('/api/v1/publishers/{publisher_slug}/workspaces', {
        params: { path: { publisher_slug: publisherSlug } },
        body,
      })
      if (error) throw new Error(friendlyError(error, 'Create failed.'))
      return body
    },
    onSuccess: (body) => {
      // Surface the group binding in the toast — admins regularly forgot
      // to PATCH it before this field was on the create form, and a
      // silent "created" toast made the omission invisible.
      const suffix = body.group_name
        ? ` (bound to ${body.group_name})`
        : ' (admin-only writes)'
      toast.success(`Workspace ${body.slug} created${suffix}`)
      setCreateOpen(false)
      invalidate()
    },
    onError: (err: Error) => setCreateError(err.message),
  })

  const patchMutation = useMutation({
    mutationFn: async ({
      workspaceSlug,
      body,
    }: {
      workspaceSlug: string
      body: { name?: string; description?: string; contact?: string; group_name?: string }
    }) => {
      setEditError(null)
      const { error } = await api.PATCH(
        '/api/v1/publishers/{publisher_slug}/workspaces/{workspace_slug}',
        {
          params: { path: { publisher_slug: publisherSlug, workspace_slug: workspaceSlug } },
          body,
        },
      )
      if (error) throw new Error(friendlyError(error, 'Update failed.'))
      return { workspaceSlug, body }
    },
    onSuccess: ({ workspaceSlug, body }) => {
      // Highlight the most useful change: setting / clearing the group binding.
      let msg = `Workspace ${workspaceSlug} updated`
      if (body.group_name === '') msg = `Workspace ${workspaceSlug} group binding cleared`
      else if (body.group_name) msg = `Workspace ${workspaceSlug} bound to ${body.group_name}`
      toast.success(msg)
      setEditingSlug(null)
      invalidate()
    },
    onError: (err: Error) => setEditError(err.message),
  })

  const deleteMutation = useMutation({
    mutationFn: async (workspaceSlug: string) => {
      setDeleteError(null)
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
      return workspaceSlug
    },
    onSuccess: (workspaceSlug) => {
      toast.success(`Workspace ${workspaceSlug} deleted`)
      invalidate()
    },
    onError: (err: Error) => setDeleteError(err.message),
  })

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold flex items-center gap-2">
          <Briefcase className="h-4 w-4" aria-hidden="true" />
          Workspaces
          <span className="text-sm font-normal text-muted-foreground">({items.length})</span>
        </h2>
        <Button size="sm" onClick={() => { setCreateOpen((v) => !v); setCreateError(null) }}>
          {createOpen ? <X className="h-4 w-4" /> : <Plus className="h-4 w-4" />}
          <span className="ml-1.5">{createOpen ? 'Cancel' : 'New workspace'}</span>
        </Button>
      </div>
      <p className="text-sm text-muted-foreground">
        Workspaces group MCP servers and agents under this publisher and bind
        each set to a Keycloak group whose members can author content.
      </p>

      {/* Delete errors don't have a form anchor — keep them as a section
          banner so the user notices the failure even if they've moved on
          to a different row. */}
      {deleteError && (
        <div role="alert" className="rounded border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {deleteError}
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
              group_name: ((fd.get('group_name') as string) || '').trim() || undefined,
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
          <div className="space-y-1">
            <Label htmlFor="ws-group">Keycloak group binding</Label>
            <Input
              id="ws-group"
              name="group_name"
              placeholder="claude-team (leave blank for admin-only)"
            />
            <p className="text-xs text-muted-foreground">
              Members of this Keycloak group can author content under the workspace. Leave blank to restrict writes to admins.
            </p>
          </div>
          {/* Inline error sits next to the submit button so the user sees
              it without losing the form state in the viewport. */}
          {createError && (
            <p role="alert" className="text-sm text-destructive">
              {createError}
            </p>
          )}
          <div className="flex gap-2 flex-wrap">
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
        <TableSkeleton rows={3} cols={5} />
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
              <TableHead className="w-8" />
              <TableHead>Slug</TableHead>
              <TableHead>Name</TableHead>
              <TableHead>Group</TableHead>
              <TableHead className="hidden md:table-cell">Updated</TableHead>
              <TableHead />
            </TableRow>
          </TableHeader>
          <TableBody>
            {items.map((ws) => {
              const isExpanded = expandedSlugs.has(ws.slug)
              return (
                <Fragment key={ws.id}>
                  <TableRow>
                    <TableCell className="pr-0">
                      <Button
                        variant="ghost"
                        size="icon"
                        className="h-7 w-7"
                        aria-label={isExpanded ? 'Collapse workspace contents' : 'Expand workspace contents'}
                        aria-expanded={isExpanded}
                        onClick={() => toggleExpanded(ws.slug)}
                      >
                        {isExpanded ? (
                          <ChevronDown className="h-4 w-4" />
                        ) : (
                          <ChevronRight className="h-4 w-4" />
                        )}
                      </Button>
                    </TableCell>
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
                    <TableCell className="text-muted-foreground hidden md:table-cell">{formatDate(ws.updated_at)}</TableCell>
                    <TableCell className="text-right">
                      <div className="inline-flex gap-1 flex-wrap justify-end">
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={() => {
                            setEditingSlug(ws.slug)
                            setEditError(null)
                          }}
                        >
                          <Pencil className="h-3.5 w-3.5" aria-hidden="true" />
                          Edit
                        </Button>
                        <DeleteButton
                          label="Delete workspace"
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
                  {isExpanded && (
                    <TableRow className="bg-muted/30 hover:bg-muted/30">
                      <TableCell colSpan={6} className="p-0">
                        <WorkspaceResources publisherSlug={publisherSlug} workspaceSlug={ws.slug} />
                      </TableCell>
                    </TableRow>
                  )}
                </Fragment>
              )
            })}
          </TableBody>
        </Table>
      )}

      {/* Edit dialog: a modal overlay so the form sits in front of the
          table instead of pushing it down. Submitting closes the dialog
          via patchMutation.onSuccess, which clears editingSlug. */}
      {editingSlug && (
        <EditWorkspaceDialog
          workspace={items.find((w) => w.slug === editingSlug)!}
          onSubmit={(body) =>
            patchMutation.mutate({ workspaceSlug: editingSlug, body })
          }
          onClose={() => {
            setEditingSlug(null)
            setEditError(null)
          }}
          isPending={patchMutation.isPending}
          error={editError}
        />
      )}
    </div>
  )
}

// Modal edit dialog for a single workspace. Mirrors the admin-layout
// mobile drawer pattern: a fixed-position overlay with a backdrop button
// (closes on click) and an inner role="dialog" panel. Body scroll is
// locked while it's open. Escape key closes too.
function EditWorkspaceDialog({
  workspace,
  onSubmit,
  onClose,
  isPending,
  error,
}: {
  workspace: Workspace
  onSubmit: (body: { name: string; description?: string; contact?: string; group_name: string }) => void
  onClose: () => void
  isPending: boolean
  error: string | null
}) {
  // Esc-to-close + body-scroll lock for the lifetime of the dialog.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    window.addEventListener('keydown', onKey)
    const prevOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    return () => {
      window.removeEventListener('keydown', onKey)
      document.body.style.overflow = prevOverflow
    }
  }, [onClose])

  return (
    <>
      {/* Backdrop — clicking it closes the dialog. Implemented as a
          button so it's keyboard-discoverable (focus-visible ring). */}
      {/* Backdrop sits at z-40 so the dialog panel (z-50) is always
          above it — matches the admin layout's mobile drawer pattern.
          DOM order alone happened to work, but a z-index split is the
          robust answer when these two ever get reordered. */}
      <button
        type="button"
        aria-label="Close edit workspace dialog"
        className="fixed inset-0 z-40 bg-background/80 backdrop-blur-sm"
        onClick={onClose}
      />
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby="edit-workspace-title"
        className="fixed left-1/2 top-1/2 z-50 w-[min(36rem,calc(100vw-2rem))] -translate-x-1/2 -translate-y-1/2 rounded-lg border bg-background shadow-lg max-h-[calc(100vh-2rem)] overflow-y-auto"
      >
        <div className="flex items-center justify-between border-b px-4 py-3">
          <h3 id="edit-workspace-title" className="text-sm font-semibold">
            Edit workspace <span className="font-mono">{workspace.slug}</span>
          </h3>
          <Button
            type="button"
            variant="ghost"
            size="icon"
            className="h-7 w-7"
            aria-label="Close"
            onClick={onClose}
          >
            <X className="h-4 w-4" />
          </Button>
        </div>
        <form
          className="space-y-4 p-4"
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
          <div className="grid gap-3 md:grid-cols-2">
            <div className="space-y-1">
              <Label htmlFor="edit-name">Name <span className="text-destructive">*</span></Label>
              <Input id="edit-name" name="name" required defaultValue={workspace.name} autoFocus />
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
          {error && (
            <p role="alert" className="text-sm text-destructive">
              {error}
            </p>
          )}
          <div className="flex gap-2 flex-wrap justify-end pt-2">
            <Button type="button" variant="outline" size="sm" onClick={onClose}>
              Cancel
            </Button>
            <Button type="submit" size="sm" disabled={isPending}>
              {isPending ? 'Saving…' : 'Save changes'}
            </Button>
          </div>
        </form>
      </div>
    </>
  )
}

// Inline expansion that lists the MCP servers and agents scoped to a single
// workspace. Two parallel queries; both are gated on the row being expanded
// (the parent only mounts this component for expanded rows).
function WorkspaceResources({
  publisherSlug,
  workspaceSlug,
}: {
  publisherSlug: string
  workspaceSlug: string
}) {
  const { accessToken } = useAuth()
  const api = useAuthClient()

  const mcp = useQuery({
    queryKey: ['admin-workspace-mcp', publisherSlug, workspaceSlug],
    queryFn: async () => {
      const r = await api.GET(
        '/api/v1/publishers/{publisher_slug}/workspaces/{workspace_slug}/servers',
        {
          params: {
            path: { publisher_slug: publisherSlug, workspace_slug: workspaceSlug },
            query: { limit: 50 },
          },
        },
      )
      return (r.data?.items ?? []) as MCPServer[]
    },
    enabled: !!accessToken,
  })

  const agents = useQuery({
    queryKey: ['admin-workspace-agents', publisherSlug, workspaceSlug],
    queryFn: async () => {
      const r = await api.GET(
        '/api/v1/publishers/{publisher_slug}/workspaces/{workspace_slug}/agents',
        {
          params: {
            path: { publisher_slug: publisherSlug, workspace_slug: workspaceSlug },
            query: { limit: 50 },
          },
        },
      )
      return (r.data?.items ?? []) as Agent[]
    },
    enabled: !!accessToken,
  })

  const isPending = mcp.isPending || agents.isPending
  const isError = mcp.isError || agents.isError
  const mcpItems = mcp.data ?? []
  const agentItems = agents.data ?? []

  return (
    <div className="px-4 py-4 space-y-4">
      {isPending ? (
        <p className="text-sm text-muted-foreground">Loading workspace contents…</p>
      ) : isError ? (
        <p className="text-sm text-destructive">Failed to load workspace contents.</p>
      ) : (
        <div className="grid gap-4 md:grid-cols-2">
          <ResourceList
            icon={<Server className="h-3.5 w-3.5" aria-hidden="true" />}
            title="MCP servers"
            count={mcpItems.length}
            emptyCopy="No MCP servers in this workspace."
            items={mcpItems.map((s) => ({
              key: s.id,
              name: s.name,
              slug: s.slug,
              status: s.status,
              updatedAt: s.updated_at,
              href: `/admin/mcp/${s.namespace}/${s.slug}`,
            }))}
          />
          <ResourceList
            icon={<Bot className="h-3.5 w-3.5" aria-hidden="true" />}
            title="Agents"
            count={agentItems.length}
            emptyCopy="No agents in this workspace."
            items={agentItems.map((a) => ({
              key: a.id,
              name: a.name,
              slug: a.slug,
              status: a.status,
              updatedAt: a.updated_at,
              href: `/admin/agents/${a.namespace}/${a.slug}`,
            }))}
          />
        </div>
      )}
    </div>
  )
}

type ResourceStatus = 'draft' | 'published' | 'deprecated'

interface ResourceItem {
  key: string
  name: string
  slug: string
  status: ResourceStatus
  updatedAt: string
  href: string
}

function ResourceList({
  icon,
  title,
  count,
  emptyCopy,
  items,
}: {
  icon: React.ReactNode
  title: string
  count: number
  emptyCopy: string
  items: ResourceItem[]
}) {
  return (
    <div className="rounded-md border bg-background">
      <div className="flex items-center gap-2 border-b px-3 py-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
        {icon}
        {title}
        <span className="font-normal normal-case">({count})</span>
      </div>
      {items.length === 0 ? (
        <p className="px-3 py-3 text-sm text-muted-foreground">{emptyCopy}</p>
      ) : (
        <ul className="divide-y">
          {items.map((it) => (
            <li key={it.key} className="flex items-center justify-between gap-2 px-3 py-2 text-sm">
              <div className="min-w-0">
                <div className="font-medium truncate">{it.name}</div>
                <div className="font-mono text-xs text-muted-foreground truncate">{it.slug}</div>
              </div>
              <div className="flex items-center gap-2 shrink-0">
                <StatusBadge status={it.status} />
                <Button variant="outline" size="sm" asChild>
                  <Link to={it.href}>
                    Manage
                    <ArrowRight className="h-3.5 w-3.5" aria-hidden="true" />
                  </Link>
                </Button>
              </div>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
