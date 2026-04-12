import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Link, useParams, useNavigate } from 'react-router-dom'
import { ArrowLeft, Package } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Badge, StatusBadge, VisibilityBadge } from '@/components/ui/badge'
import { LifecycleStepper } from '@/components/admin/lifecycle-stepper'
import { DeprecateButton } from '@/components/admin/deprecate-button'
import { DeleteButton } from '@/components/admin/delete-button'
import { Separator } from '@/components/ui/separator'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { RawJsonViewer } from '@/components/ui/raw-json-viewer'
import { InstallCommand } from '@/components/ui/install-command'
import { useAuthClient } from '@/lib/api-client'
import { formatDate, getInstallCommand, ecosystemLabel, isRemoteTransport } from '@/lib/utils'
import { useAuth } from '@/auth/AuthContext'

export default function AdminMCPDetail() {
  const { ns, slug } = useParams<{ ns: string; slug: string }>()
  const { accessToken } = useAuth()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [editOpen, setEditOpen] = useState(false)

  const api = useAuthClient()
  const { data, isPending, isError } = useQuery({
    queryKey: ['admin-mcp-detail', ns, slug],
    queryFn: () => api.GET('/api/v1/mcp/servers/{namespace}/{slug}', {
      params: { path: { namespace: ns!, slug: slug! } },
    }).then(r => r.data),
    enabled: !!ns && !!slug && !!accessToken,
  })

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ['admin-mcp-detail', ns, slug] })
    queryClient.invalidateQueries({ queryKey: ['admin-mcp'] })
  }

  const visibilityMutation = useMutation({
    mutationFn: async (newVisibility: 'public' | 'private') => {
      await api.POST('/api/v1/mcp/servers/{namespace}/{slug}/visibility', {
        params: { path: { namespace: ns!, slug: slug! } },
        body: { visibility: newVisibility },
      })
    },
    onSuccess: invalidate,
  })

  const deprecateMutation = useMutation({
    mutationFn: async () => {
      await api.POST('/api/v1/mcp/servers/{namespace}/{slug}/deprecate', {
        params: { path: { namespace: ns!, slug: slug! } },
      })
    },
    onSuccess: invalidate,
  })

  const editMutation = useMutation({
    mutationFn: async (body: { name: string; description: string; homepage_url: string; repo_url: string; license: string }) => {
      await api.PATCH('/api/v1/mcp/servers/{namespace}/{slug}', {
        params: { path: { namespace: ns!, slug: slug! } },
        body,
      })
    },
    onSuccess: () => { invalidate(); setEditOpen(false) },
  })

  const deleteMutation = useMutation({
    mutationFn: async () => {
      const { error } = await api.DELETE('/api/v1/mcp/servers/{namespace}/{slug}', {
        params: { path: { namespace: ns!, slug: slug! } },
      })
      if (error) throw new Error((error as { title?: string }).title ?? 'Delete failed')
    },
    onSuccess: () => {
      queryClient.removeQueries({ queryKey: ['admin-mcp'] })
      navigate('/admin/mcp')
    },
  })

  if (isPending) return <p className="text-muted-foreground">Loading…</p>
  if (isError || !data) return (
    <div className="space-y-4">
      <p className="text-destructive">Not found.</p>
      <Button variant="outline" size="sm" onClick={() => navigate('/admin/mcp')}>Back to MCP Servers</Button>
    </div>
  )

  const lv = data.latest_version

  return (
    <div className="space-y-6 max-w-3xl mx-auto">
      <nav aria-label="Breadcrumb" className="flex items-center gap-1.5 text-sm text-muted-foreground">
        <Link to="/admin/mcp" className="flex items-center gap-1 hover:text-foreground transition-colors">
          <ArrowLeft className="h-3.5 w-3.5" aria-hidden="true" />
          MCP Servers
        </Link>
        <span aria-hidden="true">/</span>
        <span className="font-mono text-foreground">{data.namespace}/{data.slug}</span>
      </nav>

      <div className="flex items-center gap-3 flex-wrap">
        <h1 className="text-2xl font-bold flex-1">{data.name}</h1>
        <div className="flex gap-2">
          {lv && <Badge variant="outline" className="font-mono">v{lv.version}</Badge>}
          <StatusBadge status={data.status} />
          <VisibilityBadge visibility={data.visibility} />
        </div>
      </div>

      <LifecycleStepper
        currentStatus={data.status}
        onTransition={(target) => {
          if (target === 'deprecated') deprecateMutation.mutate()
          // Other transitions (e.g., publish from draft) are handled per-version
        }}
      />

      <dl className="grid grid-cols-2 gap-x-4 gap-y-2 text-sm">
        <dt className="text-muted-foreground">Namespace / Slug</dt>
        <dd className="font-mono">{data.namespace}/{data.slug}</dd>
        {data.description && (
          <>
            <dt className="text-muted-foreground">Description</dt>
            <dd>{data.description}</dd>
          </>
        )}
        {lv && (
          <>
            <dt className="text-muted-foreground">Runtime</dt>
            <dd><Badge variant="secondary">{lv.runtime}</Badge></dd>
            <dt className="text-muted-foreground">Protocol version</dt>
            <dd className="font-mono">{lv.protocol_version}</dd>
            {lv.published_at && (
              <>
                <dt className="text-muted-foreground">Published</dt>
                <dd>{formatDate(lv.published_at)}</dd>
              </>
            )}
          </>
        )}
        {data.license && (
          <>
            <dt className="text-muted-foreground">License</dt>
            <dd>{data.license}</dd>
          </>
        )}
        <dt className="text-muted-foreground">Created</dt>
        <dd>{formatDate(data.created_at)}</dd>
        <dt className="text-muted-foreground">Updated</dt>
        <dd>{formatDate(data.updated_at)}</dd>
      </dl>

      {/* Packages */}
      {lv?.packages && lv.packages.length > 0 && (
        <div className="space-y-3">
          <h2 className="text-lg font-semibold flex items-center gap-2">
            <Package className="h-4 w-4" aria-hidden="true" /> Packages
          </h2>
          <div className="space-y-4">
            {lv.packages.map((pkg, i) => {
              const remote = isRemoteTransport(pkg.transport.type)
              return (
                <div key={i} className="space-y-1.5">
                  <div className="flex items-center gap-2 flex-wrap">
                    <Badge variant="secondary" className="text-xs">
                      {ecosystemLabel(pkg.registryType)}
                    </Badge>
                    <span className="text-xs text-muted-foreground font-mono">
                      {pkg.identifier}@{pkg.version}
                    </span>
                    <Badge variant="outline" className="text-xs">
                      {pkg.transport.type}
                    </Badge>
                  </div>
                  {remote && pkg.transport.url ? (
                    <div className="space-y-1">
                      <p className="text-xs text-muted-foreground">Endpoint URL</p>
                      <InstallCommand command={pkg.transport.url} />
                    </div>
                  ) : (
                    <div className="space-y-1">
                      <p className="text-xs text-muted-foreground">Run command</p>
                      <InstallCommand command={getInstallCommand(pkg)} />
                    </div>
                  )}
                </div>
              )
            })}
          </div>
        </div>
      )}

      <Separator />

      {/* Edit form */}
      {editOpen && (
        <form
          className="space-y-4 border rounded-lg p-4"
          onSubmit={(e) => {
            e.preventDefault()
            const fd = new FormData(e.currentTarget)
            editMutation.mutate({
              name: fd.get('name') as string,
              description: fd.get('description') as string,
              homepage_url: fd.get('homepage_url') as string,
              repo_url: fd.get('repo_url') as string,
              license: fd.get('license') as string,
            })
          }}
        >
          <h2 className="text-lg font-semibold">Edit MCP Server</h2>
          <div className="grid gap-3">
            <div className="space-y-1">
              <Label htmlFor="name">Name <span className="text-destructive">*</span></Label>
              <Input id="name" name="name" defaultValue={data.name} required />
            </div>
            <div className="space-y-1">
              <Label htmlFor="description">Description</Label>
              <Input id="description" name="description" defaultValue={data.description ?? ''} />
            </div>
            <div className="space-y-1">
              <Label htmlFor="homepage_url">Homepage URL</Label>
              <Input id="homepage_url" name="homepage_url" type="url" defaultValue={data.homepage_url ?? ''} />
            </div>
            <div className="space-y-1">
              <Label htmlFor="repo_url">Repository URL</Label>
              <Input id="repo_url" name="repo_url" type="url" defaultValue={data.repo_url ?? ''} />
            </div>
            <div className="space-y-1">
              <Label htmlFor="license">License</Label>
              <Input id="license" name="license" defaultValue={data.license ?? ''} />
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

      <div className="space-y-3">
        <h2 className="text-lg font-semibold">Actions</h2>
        <div className="flex flex-wrap gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={() => setEditOpen(v => !v)}
          >
            {editOpen ? 'Cancel edit' : 'Edit'}
          </Button>

          <Button
            variant="outline"
            size="sm"
            disabled={visibilityMutation.isPending}
            onClick={() => visibilityMutation.mutate(data.visibility === 'public' ? 'private' : 'public')}
          >
            Make {data.visibility === 'public' ? 'private' : 'public'}
          </Button>

          {data.status === 'published' && (
            <DeprecateButton
              onDeprecate={() => deprecateMutation.mutate()}
              entityName={data.name}
            />
          )}

          <DeleteButton
            onDelete={() => deleteMutation.mutate()}
            entityName={data.name}
            isPending={deleteMutation.isPending}
          />
        </div>
        {(visibilityMutation.isError || deprecateMutation.isError || deleteMutation.isError) && (
          <p className="text-sm text-destructive">Action failed. Please try again.</p>
        )}
      </div>

      <Separator />

      <RawJsonViewer data={data} title="Raw API response" />
    </div>
  )
}
