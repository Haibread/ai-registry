import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Link, useParams, useNavigate } from 'react-router-dom'
import { ArrowLeft, CheckCircle2, Circle, Server, Bot } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Separator } from '@/components/ui/separator'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { StatusBadge } from '@/components/ui/badge'
import { DeleteButton } from '@/components/admin/delete-button'
import { useAuthClient } from '@/lib/api-client'
import { formatDate } from '@/lib/utils'
import { useAuth } from '@/auth/AuthContext'

export default function AdminPublisherDetail() {
  const { slug } = useParams<{ slug: string }>()
  const { accessToken } = useAuth()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [editOpen, setEditOpen] = useState(false)

  const api = useAuthClient()

  const { data: publisher, isPending, isError } = useQuery({
    queryKey: ['admin-publisher', slug],
    queryFn: () => api.GET('/api/v1/publishers/{slug}', {
      params: { path: { slug: slug! } },
    }).then(r => r.data),
    enabled: !!slug && !!accessToken,
  })

  const { data: mcpData } = useQuery({
    queryKey: ['admin-publisher-mcp', slug],
    queryFn: () => api.GET('/api/v1/mcp/servers', {
      params: { query: { namespace: slug, limit: 50 } },
    }).then(r => r.data),
    enabled: !!slug && !!accessToken,
  })

  const { data: agentsData } = useQuery({
    queryKey: ['admin-publisher-agents', slug],
    queryFn: () => api.GET('/api/v1/agents', {
      params: { query: { namespace: slug, limit: 50 } },
    }).then(r => r.data),
    enabled: !!slug && !!accessToken,
  })

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ['admin-publisher', slug] })
    queryClient.invalidateQueries({ queryKey: ['admin-publishers'] })
  }

  const editMutation = useMutation({
    mutationFn: async (body: { name: string; contact: string }) => {
      await api.PATCH('/api/v1/publishers/{slug}', {
        params: { path: { slug: slug! } },
        body,
      })
    },
    onSuccess: () => { invalidate(); setEditOpen(false) },
  })

  const deleteMutation = useMutation({
    mutationFn: async () => {
      const { error } = await api.DELETE('/api/v1/publishers/{slug}', {
        params: { path: { slug: slug! } },
      })
      if (error) throw new Error((error as { title?: string }).title ?? 'Delete failed')
    },
    onSuccess: () => {
      queryClient.removeQueries({ queryKey: ['admin-publishers'] })
      navigate('/admin/publishers')
    },
  })

  const mcpServers = mcpData?.items ?? []
  const agents = agentsData?.items ?? []

  if (isPending) return <p className="text-muted-foreground">Loading…</p>
  if (isError || !publisher) return (
    <div className="space-y-4">
      <p className="text-destructive">Not found.</p>
      <Button variant="outline" size="sm" onClick={() => navigate('/admin/publishers')}>Back to Publishers</Button>
    </div>
  )

  return (
    <div className="space-y-6 max-w-4xl mx-auto">
      <nav aria-label="Breadcrumb" className="flex items-center gap-1.5 text-sm text-muted-foreground">
        <Link to="/admin/publishers" className="flex items-center gap-1 hover:text-foreground transition-colors">
          <ArrowLeft className="h-3.5 w-3.5" aria-hidden="true" />
          Publishers
        </Link>
        <span aria-hidden="true">/</span>
        <span className="font-mono text-foreground">{publisher.slug}</span>
      </nav>

      {/* Header */}
      <div className="flex items-start gap-3 flex-wrap">
        <div className="flex-1">
          <h1 className="text-2xl font-bold">{publisher.name}</h1>
          <p className="text-sm text-muted-foreground font-mono mt-0.5">{publisher.slug}</p>
        </div>
        {publisher.verified ? (
          <Badge variant="success" className="gap-1">
            <CheckCircle2 className="h-3 w-3" aria-hidden="true" /> Verified
          </Badge>
        ) : (
          <Badge variant="muted" className="gap-1">
            <Circle className="h-3 w-3" aria-hidden="true" /> Unverified
          </Badge>
        )}
      </div>

      {/* Metadata */}
      <dl className="grid grid-cols-2 gap-x-4 gap-y-2 text-sm max-w-sm">
        {publisher.contact && (
          <>
            <dt className="text-muted-foreground">Contact</dt>
            <dd>{publisher.contact}</dd>
          </>
        )}
        <dt className="text-muted-foreground">Created</dt>
        <dd>{formatDate(publisher.created_at)}</dd>
        <dt className="text-muted-foreground">Updated</dt>
        <dd>{formatDate(publisher.updated_at)}</dd>
      </dl>

      {/* Edit form */}
      {editOpen && (
        <form
          className="space-y-4 border rounded-lg p-4 max-w-sm"
          onSubmit={(e) => {
            e.preventDefault()
            const fd = new FormData(e.currentTarget)
            editMutation.mutate({
              name: fd.get('name') as string,
              contact: fd.get('contact') as string,
            })
          }}
        >
          <h2 className="text-lg font-semibold">Edit Publisher</h2>
          <div className="grid gap-3">
            <div className="space-y-1">
              <Label htmlFor="name">Name <span className="text-destructive">*</span></Label>
              <Input id="name" name="name" defaultValue={publisher.name} required />
            </div>
            <div className="space-y-1">
              <Label htmlFor="contact">Contact</Label>
              <Input id="contact" name="contact" defaultValue={publisher.contact ?? ''} />
            </div>
          </div>
          {editMutation.isError && (
            <p className="text-sm text-destructive">Update failed. Please try again.</p>
          )}
          <div className="flex gap-2">
            <Button type="submit" size="sm" disabled={editMutation.isPending}>
              {editMutation.isPending ? 'Saving…' : 'Save changes'}
            </Button>
            <Button type="button" variant="outline" size="sm" onClick={() => setEditOpen(false)}>
              Cancel
            </Button>
          </div>
        </form>
      )}

      {/* Actions */}
      <div className="space-y-3">
        <h2 className="text-lg font-semibold">Actions</h2>
        <div className="flex flex-wrap gap-2">
          <Button variant="outline" size="sm" onClick={() => setEditOpen(v => !v)}>
            {editOpen ? 'Cancel edit' : 'Edit'}
          </Button>
          <DeleteButton
            onDelete={() => deleteMutation.mutate()}
            entityName={publisher.name}
            isPending={deleteMutation.isPending}
          />
        </div>
        {deleteMutation.isError && (
          <p className="text-sm text-destructive">
            Delete failed — publisher may still have active entries.
          </p>
        )}
      </div>

      <Separator />

      {/* MCP Servers */}
      <div className="space-y-3">
        <div className="flex items-center justify-between">
          <h2 className="text-lg font-semibold flex items-center gap-2">
            <Server className="h-4 w-4" aria-hidden="true" />
            MCP Servers
            <span className="text-sm font-normal text-muted-foreground">({mcpServers.length})</span>
          </h2>
          <Button size="sm" asChild>
            <Link to="/admin/mcp/new">New Server</Link>
          </Button>
        </div>

        {mcpServers.length === 0 ? (
          <p className="text-sm text-muted-foreground py-4">No MCP servers under this namespace.</p>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Slug</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Updated</TableHead>
                <TableHead />
              </TableRow>
            </TableHeader>
            <TableBody>
              {mcpServers.map((s) => (
                <TableRow key={s.id}>
                  <TableCell className="font-medium">{s.name}</TableCell>
                  <TableCell className="font-mono text-sm text-muted-foreground">{s.slug}</TableCell>
                  <TableCell><StatusBadge status={s.status} /></TableCell>
                  <TableCell className="text-muted-foreground">{formatDate(s.updated_at)}</TableCell>
                  <TableCell>
                    <Button variant="ghost" size="sm" asChild>
                      <Link to={`/admin/mcp/${s.namespace}/${s.slug}`}>Manage</Link>
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </div>

      <Separator />

      {/* Agents */}
      <div className="space-y-3">
        <div className="flex items-center justify-between">
          <h2 className="text-lg font-semibold flex items-center gap-2">
            <Bot className="h-4 w-4" aria-hidden="true" />
            Agents
            <span className="text-sm font-normal text-muted-foreground">({agents.length})</span>
          </h2>
          <Button size="sm" asChild>
            <Link to="/admin/agents/new">New Agent</Link>
          </Button>
        </div>

        {agents.length === 0 ? (
          <p className="text-sm text-muted-foreground py-4">No agents under this namespace.</p>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Slug</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Updated</TableHead>
                <TableHead />
              </TableRow>
            </TableHeader>
            <TableBody>
              {agents.map((a) => (
                <TableRow key={a.id}>
                  <TableCell className="font-medium">{a.name}</TableCell>
                  <TableCell className="font-mono text-sm text-muted-foreground">{a.slug}</TableCell>
                  <TableCell><StatusBadge status={a.status} /></TableCell>
                  <TableCell className="text-muted-foreground">{formatDate(a.updated_at)}</TableCell>
                  <TableCell>
                    <Button variant="ghost" size="sm" asChild>
                      <Link to={`/admin/agents/${a.namespace}/${a.slug}`}>Manage</Link>
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
