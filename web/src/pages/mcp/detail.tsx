import { useQuery } from '@tanstack/react-query'
import { Link, useParams } from 'react-router-dom'
import { ArrowLeft, ExternalLink, Package } from 'lucide-react'
import { Header } from '@/components/layout/header'
import { Footer } from '@/components/layout/footer'
import { Badge, StatusBadge, VisibilityBadge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Separator } from '@/components/ui/separator'
import { RawJsonViewer } from '@/components/ui/raw-json-viewer'
import { InstallCommand } from '@/components/ui/install-command'
import { getPublicClient } from '@/lib/api-client'
import { formatDate, getInstallCommand, ecosystemLabel, isRemoteTransport } from '@/lib/utils'

export default function MCPDetailPage() {
  const { ns, slug } = useParams<{ ns: string; slug: string }>()
  const api = getPublicClient()
  const { data, isLoading, isError } = useQuery({
    queryKey: ['mcp-server', ns, slug],
    queryFn: () => api.GET('/api/v1/mcp/servers/{namespace}/{slug}', {
      params: { path: { namespace: ns!, slug: slug! } },
    }).then(r => r.data),
    enabled: !!ns && !!slug,
  })

  if (isLoading) return (
    <div className="flex min-h-screen flex-col">
      <Header />
      <main className="flex-1 container py-8"><p className="text-muted-foreground">Loading…</p></main>
      <Footer />
    </div>
  )
  if (isError || !data) return (
    <div className="flex min-h-screen flex-col">
      <Header />
      <main className="flex-1 container py-8"><p className="text-destructive">Not found.</p></main>
      <Footer />
    </div>
  )

  const lv = data.latest_version

  return (
    <div className="flex min-h-screen flex-col">
      <Header />
      <main className="flex-1 container py-8 max-w-3xl space-y-6">
        <nav aria-label="Breadcrumb" className="flex items-center gap-1.5 text-sm text-muted-foreground">
          <Link to="/mcp" className="flex items-center gap-1 hover:text-foreground transition-colors">
            <ArrowLeft className="h-3.5 w-3.5" aria-hidden="true" /> MCP Servers
          </Link>
          <span aria-hidden="true">/</span>
          <span className="font-mono text-foreground">{data.namespace}/{data.slug}</span>
        </nav>

        <div className="space-y-2">
          <div className="flex items-start gap-3 flex-wrap">
            <h1 className="text-3xl font-bold flex-1">{data.name}</h1>
            <div className="flex gap-2 flex-wrap">
              {lv && <Badge variant="outline" className="font-mono">v{lv.version}</Badge>}
              <StatusBadge status={data.status} />
              <VisibilityBadge visibility={data.visibility} />
            </div>
          </div>
          <p className="text-sm text-muted-foreground font-mono">{data.namespace}/{data.slug}</p>
        </div>

        {data.description && <p className="text-muted-foreground">{data.description}</p>}

        <Separator />

        <dl className="grid grid-cols-2 gap-4 text-sm">
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

        {lv?.packages && lv.packages.length > 0 && (
          <div className="space-y-3">
            <h2 className="text-lg font-semibold flex items-center gap-2">
              <Package className="h-4 w-4" aria-hidden="true" />
              {lv.packages.every(p => isRemoteTransport(p.transport.type)) ? 'Connection' : 'Installation'}
            </h2>
            <div className="space-y-4">
              {lv.packages.map((pkg, i) => {
                const remote = isRemoteTransport(pkg.transport.type)
                return (
                  <div key={i} className="space-y-1.5">
                    <div className="flex items-center gap-2 flex-wrap">
                      <Badge variant="secondary" className="text-xs">{ecosystemLabel(pkg.registryType)}</Badge>
                      <span className="text-xs text-muted-foreground font-mono">{pkg.identifier}@{pkg.version}</span>
                      <Badge variant="outline" className="text-xs">{pkg.transport.type}</Badge>
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

        <div className="flex gap-3 flex-wrap">
          {data.repo_url && (
            <Button variant="outline" size="sm" asChild>
              <a href={data.repo_url} target="_blank" rel="noopener noreferrer" className="flex items-center gap-1.5">
                <ExternalLink className="h-4 w-4" /> Repository
              </a>
            </Button>
          )}
          {data.homepage_url && (
            <Button variant="outline" size="sm" asChild>
              <a href={data.homepage_url} target="_blank" rel="noopener noreferrer" className="flex items-center gap-1.5">
                <ExternalLink className="h-4 w-4" /> Homepage
              </a>
            </Button>
          )}
        </div>

        <Separator />
        <RawJsonViewer data={data} title="Raw API response" />
      </main>
      <Footer />
    </div>
  )
}
