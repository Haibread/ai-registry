import { useQuery } from '@tanstack/react-query'
import { Link, useParams } from 'react-router-dom'
import { ExternalLink, Package } from 'lucide-react'
import { Header } from '@/components/layout/header'
import { Footer } from '@/components/layout/footer'
import { Badge, StatusBadge, VisibilityBadge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Separator } from '@/components/ui/separator'
import { RawJsonViewer } from '@/components/ui/raw-json-viewer'
import { InstallCommand } from '@/components/ui/install-command'
import { Breadcrumbs } from '@/components/ui/breadcrumbs'
import { DetailPageSkeleton } from '@/components/ui/detail-page-skeleton'
import { EmptyState } from '@/components/ui/empty-state'
import { TooltipInfo } from '@/components/ui/tooltip-info'
import { CopyButton } from '@/components/ui/copy-button'
import { ResourceIcon } from '@/components/ui/resource-icon'
import { getPublicClient } from '@/lib/api-client'
import { formatDate, getInstallCommand, ecosystemLabel, isRemoteTransport } from '@/lib/utils'
import { getFieldExplanation } from '@/lib/field-explanations'

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
      <main className="flex-1 container py-8 max-w-3xl">
        <DetailPageSkeleton />
      </main>
      <Footer />
    </div>
  )
  if (isError || !data) return (
    <div className="flex min-h-screen flex-col">
      <Header />
      <main className="flex-1 container py-8 max-w-3xl">
        <EmptyState
          icon={<ResourceIcon type="mcp-server" className="h-10 w-10" />}
          title="Server not found"
          description="The MCP server you're looking for doesn't exist or has been removed."
          action={<Button variant="outline" size="sm" asChild><Link to="/mcp">Back to MCP Servers</Link></Button>}
        />
      </main>
      <Footer />
    </div>
  )

  const lv = data.latest_version

  return (
    <div className="flex min-h-screen flex-col">
      <Header />
      <main className="flex-1 container py-8 max-w-3xl space-y-6">
        <Breadcrumbs
          segments={[
            { label: 'Home', href: '/' },
            { label: 'MCP Servers', href: '/mcp' },
            { label: data.namespace, href: `/mcp?namespace=${data.namespace}` },
            { label: data.slug },
          ]}
        />

        <div className="space-y-2">
          <div className="flex items-start gap-3 flex-wrap">
            <h1 className="text-2xl sm:text-3xl font-bold flex-1 min-w-0 break-words">{data.name}</h1>
            <div className="flex gap-2 flex-wrap">
              {lv && <Badge variant="outline" className="font-mono">v{lv.version}</Badge>}
              <StatusBadge status={data.status} />
              <VisibilityBadge visibility={data.visibility} />
            </div>
          </div>
          <div className="flex items-center gap-2">
            <p className="text-sm text-muted-foreground font-mono">
              <Link to={`/mcp?namespace=${data.namespace}`} className="hover:text-foreground transition-colors">
                {data.namespace}
              </Link>
              /{data.slug}
            </p>
            <CopyButton value={`${data.namespace}/${data.slug}`} label="Copy identifier" />
          </div>
        </div>

        {data.description && <p className="text-muted-foreground">{data.description}</p>}

        <Separator />

        <dl className="grid grid-cols-1 sm:grid-cols-2 gap-4 text-sm">
          {lv && (
            <>
              <dt className="text-muted-foreground flex items-center gap-1">
                Runtime
                <TooltipInfo content={getFieldExplanation('runtime') ?? ''} />
              </dt>
              <dd>
                <Badge variant="secondary">{lv.runtime}</Badge>
                {getFieldExplanation(lv.runtime) && (
                  <TooltipInfo content={getFieldExplanation(lv.runtime)!} className="ml-1" />
                )}
              </dd>
              <dt className="text-muted-foreground flex items-center gap-1">
                Protocol version
                <TooltipInfo content={getFieldExplanation('protocol_version') ?? ''} />
              </dt>
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
                      <span className="text-xs text-muted-foreground font-mono truncate">{pkg.identifier}@{pkg.version}</span>
                      <Badge variant="outline" className="text-xs">{pkg.transport.type}</Badge>
                      {getFieldExplanation(pkg.transport.type) && (
                        <TooltipInfo content={getFieldExplanation(pkg.transport.type)!} />
                      )}
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
