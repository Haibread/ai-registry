import { useRef } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link, useParams, useLocation, useNavigate } from 'react-router-dom'
import {
  ExternalLink,
  Package,
  Package2,
  Cpu,
  Code2,
  CalendarClock,
  Scale,
  Link2,
  Shield,
} from 'lucide-react'
import { Header } from '@/components/layout/header'
import { Footer } from '@/components/layout/footer'
import { Badge, StatusBadge, VisibilityBadge, VerifiedBadge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '@/components/ui/card'
import { Separator } from '@/components/ui/separator'
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs'
import { RawJsonViewer } from '@/components/ui/raw-json-viewer'
import { InstallCommand } from '@/components/ui/install-command'
import { Breadcrumbs } from '@/components/ui/breadcrumbs'
import { DetailPageSkeleton } from '@/components/ui/detail-page-skeleton'
import { EmptyState } from '@/components/ui/empty-state'
import { TooltipInfo } from '@/components/ui/tooltip-info'
import { CopyButton } from '@/components/ui/copy-button'
import { ResourceIcon } from '@/components/ui/resource-icon'
import { FreshnessIndicator } from '@/components/ui/freshness-indicator'
import { CapabilitiesSection } from '@/components/mcp/capabilities-section'
import { MCPConfigGenerator } from '@/components/mcp/config-generator'
import { MarkdownRenderer } from '@/components/ui/markdown-renderer'
import { PublisherSidebar } from '@/components/shared/publisher-sidebar'
import { StatTile } from '@/components/shared/stat-tile'
import { SectionHeader } from '@/components/shared/section-header'
import { EngagementStrip } from '@/components/shared/engagement-strip'
import { ActivityFeed } from '@/components/shared/activity-feed'
import { RelatedEntries } from '@/components/shared/related-entries'
import { VersionHistory } from '@/components/shared/version-history'
import { StickyDetailHeader } from '@/components/shared/sticky-detail-header'
import { ReportDialog } from '@/components/shared/report-dialog'
import { useRecordView, useRecordCopy } from '@/hooks/use-record-event'
import { getPublicClient } from '@/lib/api-client'
import { formatDate, getInstallCommand, ecosystemLabel, isRemoteTransport } from '@/lib/utils'
import { getFieldExplanation } from '@/lib/field-explanations'

export default function MCPDetailPage() {
  const { ns, slug } = useParams<{ ns: string; slug: string }>()
  const location = useLocation()
  const navigate = useNavigate()
  const api = getPublicClient()
  const { data, isLoading, isError } = useQuery({
    queryKey: ['mcp-server', ns, slug],
    queryFn: () => api.GET('/api/v1/mcp/servers/{namespace}/{slug}', {
      params: { path: { namespace: ns!, slug: slug! } },
    }).then(r => r.data),
    enabled: !!ns && !!slug,
  })

  // Tab state synced to URL hash
  const defaultTab = location.hash?.replace('#', '') || 'overview'
  const handleTabChange = (value: string) => {
    navigate(`${location.pathname}#${value}`, { replace: true })
  }

  // Hooks must run unconditionally (Rules of Hooks): declare refs and
  // tracking hooks before any early returns so hook order is stable across
  // loading → loaded transitions.
  const titleRef = useRef<HTMLHeadingElement>(null)
  useRecordView('mcp', data?.namespace, data?.slug)
  const recordCopy = useRecordCopy('mcp', data?.namespace, data?.slug)

  if (isLoading) return (
    <div className="flex min-h-screen flex-col">
      <Header />
      <main className="flex-1 container py-8">
        <DetailPageSkeleton />
      </main>
      <Footer />
    </div>
  )
  if (isError || !data) return (
    <div className="flex min-h-screen flex-col">
      <Header />
      <main className="flex-1 container py-8">
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
  const capabilities = (lv as Record<string, unknown> | undefined)?.capabilities as Record<string, unknown> | undefined
  // Packages that connect to a remote URL rather than running locally.
  // For these, the endpoint URL is the primary thing a caller needs — we
  // surface it in the Overview so users don't have to jump to the Installation
  // tab to see how to connect.
  const remotePackages = (lv?.packages ?? []).filter(
    (p) => isRemoteTransport(p.transport.type) && !!p.transport.url,
  )
  const hasRemote = remotePackages.length > 0

  return (
    <div className="flex min-h-screen flex-col">
      <Header />
      <StickyDetailHeader
        type="mcp-server"
        name={data.name}
        version={lv?.version}
        identifier={`${data.namespace}/${data.slug}`}
        titleRef={titleRef}
      />
      <main className="flex-1 container py-8 space-y-6">
        <Breadcrumbs
          segments={[
            { label: 'Home', href: '/' },
            { label: 'MCP Servers', href: '/mcp' },
            { label: data.namespace, href: `/mcp/${data.namespace}` },
            { label: data.slug },
          ]}
        />

        {/* Title row */}
        <div className="space-y-2">
          <div className="flex items-center gap-3 flex-wrap">
            <h1 ref={titleRef} className="text-2xl sm:text-3xl font-bold min-w-0 break-words">{data.name}</h1>
            <div className="flex items-center gap-2 flex-wrap">
              {lv && <Badge variant="outline" className="font-mono">v{lv.version}</Badge>}
              {data.verified && <VerifiedBadge />}
              <StatusBadge status={data.status} />
              <VisibilityBadge visibility={data.visibility} />
            </div>
          </div>
          <div className="flex items-center gap-2 flex-wrap">
            <p className="text-sm text-muted-foreground font-mono">
              <Link to={`/mcp/${data.namespace}`} className="hover:text-foreground transition-colors">
                {data.namespace}
              </Link>
              /{data.slug}
            </p>
            <CopyButton value={`${data.namespace}/${data.slug}`} label="Copy identifier" />
            {(data.repo_url || data.homepage_url) && (
              <span className="h-4 w-px bg-border mx-1" aria-hidden="true" />
            )}
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
            <ReportDialog
              resourceType="mcp_server"
              resourceId={data.id}
              resourceLabel={`${data.namespace}/${data.slug}`}
            />
          </div>
        </div>

        {data.description && <p className="text-muted-foreground max-w-prose">{data.description}</p>}

        {/* README — the publisher's narrative description. Rendered near
            the top of the page (above the tabs) so it's always visible,
            regardless of which tab the reader has open. Fills the page
            container width — MarkdownRenderer already sets max-w-none. */}
        {data.readme && <MarkdownRenderer content={data.readme} />}

        <Separator />

        {/* Tabbed content */}
        <Tabs defaultValue={defaultTab} onValueChange={handleTabChange}>
          <TabsList>
            <TabsTrigger value="overview">Overview</TabsTrigger>
            <TabsTrigger value="installation">Installation</TabsTrigger>
            <TabsTrigger value="tools">
              Tools{lv?.tools && lv.tools.length > 0 ? ` (${lv.tools.length})` : ''}
            </TabsTrigger>
            <TabsTrigger value="versions">Versions</TabsTrigger>
            <TabsTrigger value="json">JSON</TabsTrigger>
          </TabsList>

          {/* ── Overview Tab ── */}
          {/* mt-6 overrides the TabsContent default mt-2 so the gap from the
              tabs to the first child matches the `space-y-6` rhythm below. */}
          <TabsContent value="overview" className="mt-6 space-y-8">
            {/* Publisher banner */}
            <PublisherSidebar namespace={data.namespace} />

            {/* ─── Connection & Runtime ───
                Primary "what is this server and how do I talk to it" card.
                For remote servers (http / sse / streamable_http) the endpoint
                URL is rendered as a hero row at the top so users don't have
                to dig into the Installation tab to find it. Runtime, protocol
                version, and capabilities round out the technical surface. */}
            <section className="space-y-3">
              <SectionHeader
                icon={hasRemote ? <Link2 /> : <Cpu />}
                title={hasRemote ? 'Connection & Runtime' : 'Runtime & Capabilities'}
              />
              <div className="rounded-xl border bg-card overflow-hidden shadow-xs">
                {/* Endpoint URL hero rows — one per remote package.
                    Most servers have a single package; when multiple exist
                    we stack them so every connection target is visible. */}
                {remotePackages.map((pkg, i) => (
                  <div
                    key={`endpoint-${i}`}
                    className={i < remotePackages.length - 1 ? 'border-b' : ''}
                  >
                    <StatTile
                      className="px-5 py-4"
                      label={remotePackages.length > 1 ? `Endpoint URL (${pkg.transport.type})` : 'Endpoint URL'}
                      icon={<Link2 />}
                      tooltip={getFieldExplanation('endpoint_url') ?? undefined}
                    >
                      <div className="flex items-center gap-2 min-w-0">
                        <a
                          href={pkg.transport.url}
                          target="_blank"
                          rel="noopener noreferrer"
                          className="font-mono text-xs hover:underline truncate"
                        >
                          {pkg.transport.url}
                        </a>
                        <CopyButton
                          value={pkg.transport.url!}
                          label="Copy endpoint URL"
                          onCopy={recordCopy}
                        />
                      </div>
                    </StatTile>
                  </div>
                ))}
                <div
                  className={`flex flex-col sm:flex-row sm:divide-x divide-y sm:divide-y-0${
                    hasRemote ? ' border-t' : ''
                  }`}
                >
                  {lv && (
                    <StatTile
                      className="flex-1 px-5 py-4"
                      label={hasRemote ? 'Transport' : 'Runtime'}
                      icon={<Cpu />}
                      tooltip={getFieldExplanation('runtime') ?? undefined}
                    >
                      <div className="flex items-center gap-1">
                        <Badge variant="secondary">{lv.runtime}</Badge>
                        {getFieldExplanation(lv.runtime) && (
                          <TooltipInfo content={getFieldExplanation(lv.runtime)!} />
                        )}
                      </div>
                    </StatTile>
                  )}
                  {lv && (
                    <StatTile
                      className="flex-1 px-5 py-4"
                      label="Protocol version"
                      icon={<Code2 />}
                      tooltip={getFieldExplanation('protocol_version') ?? undefined}
                    >
                      <span className="font-mono">{lv.protocol_version}</span>
                    </StatTile>
                  )}
                  {hasRemote && (
                    <StatTile
                      className="flex-1 px-5 py-4"
                      label="Authentication"
                      icon={<Shield />}
                      tooltip={getFieldExplanation('mcp_authentication') ?? undefined}
                    >
                      <span className="text-muted-foreground text-xs">
                        Per MCP spec (OAuth 2.1)
                      </span>
                    </StatTile>
                  )}
                </div>
                {capabilities && Object.keys(capabilities).length > 0 && (
                  <div className="border-t px-5 py-4 space-y-2">
                    <div className="text-[10px] uppercase tracking-wider font-semibold text-muted-foreground/80">
                      Capabilities
                    </div>
                    <CapabilitiesSection capabilities={capabilities} hideTitle />
                  </div>
                )}
              </div>
            </section>

            {/* ─── Release ───
                Version-level facts that are static across a single release. */}
            <section className="space-y-3">
              <SectionHeader icon={<Package2 />} title="Release" />
              <div className="rounded-xl border bg-card overflow-hidden shadow-xs">
                <div className="flex flex-col sm:flex-row sm:divide-x divide-y sm:divide-y-0">
                  <StatTile
                    className="flex-1 px-5 py-4"
                    label="Published"
                    icon={<CalendarClock />}
                  >
                    {lv?.published_at ? (
                      <div className="flex items-center gap-2 flex-wrap">
                        <span>{formatDate(lv.published_at)}</span>
                        <FreshnessIndicator updatedAt={lv.published_at} />
                      </div>
                    ) : (
                      <span className="text-muted-foreground">Draft</span>
                    )}
                  </StatTile>
                  <StatTile
                    className="flex-1 px-5 py-4"
                    label="License"
                    icon={<Scale />}
                  >
                    {data.license || <span className="text-muted-foreground">—</span>}
                  </StatTile>
                </div>
              </div>
            </section>

            {/* ─── Engagement ───
                Low-priority engagement numbers + timestamps rendered as a
                compact inline strip. No card, no emphasis — these are here
                for reference, not as the page's headline. */}
            <EngagementStrip
              viewCount={data.view_count ?? 0}
              copyCount={data.copy_count ?? 0}
              createdAt={data.created_at}
              updatedAt={data.updated_at}
            />

            {/* ─── Activity ───
                Privacy-scrubbed lifecycle feed: creation, publishes,
                deprecations, visibility changes. Backed by the public
                per-resource /activity endpoint so it stays in sync with
                the audit log without exposing actor identity. */}
            <ActivityFeed
              resourceType="mcp"
              namespace={ns}
              slug={slug}
            />
          </TabsContent>

          {/* ── Installation Tab ── */}
          <TabsContent value="installation" className="mt-6 space-y-6 max-w-3xl">
            {lv?.packages && lv.packages.length > 0 ? (
              <>
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
                              <InstallCommand command={pkg.transport.url} onCopy={recordCopy} />
                            </div>
                          ) : (
                            <div className="space-y-1">
                              <p className="text-xs text-muted-foreground">Run command</p>
                              <InstallCommand command={getInstallCommand(pkg)} onCopy={recordCopy} />
                            </div>
                          )}
                        </div>
                      )
                    })}
                  </div>
                </div>
                <Separator />
                <MCPConfigGenerator
                  serverName={data.slug}
                  packages={lv.packages.map((p) => ({
                    registryType: p.registryType,
                    identifier: p.identifier,
                    version: p.version,
                    transport: { type: p.transport.type, url: p.transport.url },
                  }))}
                />
              </>
            ) : (
              <EmptyState
                icon={<Package className="h-8 w-8 text-muted-foreground" />}
                title="No packages available"
                description="This server has no published packages yet."
              />
            )}
          </TabsContent>

          {/* ── Tools Tab ──
              Renders the first-class `tools[]` array from the latest version.
              This is NOT the `capabilities.tools` flag (which is the MCP spec's
              capability-negotiation object `{listChanged:bool}`). The array is
              publisher-declared via the admin API; the MCP spec's `tools/list`
              method would return these at runtime for clients that don't have
              access to the registry metadata. */}
          <TabsContent value="tools" className="mt-6 space-y-4">
            {lv?.tools && lv.tools.length > 0 ? (
              <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
                {lv.tools.map((tool) => (
                  <Card key={tool.name} className="bg-muted/30">
                    <CardHeader className="pb-2 pt-4 px-4">
                      <CardTitle className="text-sm flex items-center gap-2 font-mono">
                        <Cpu className="h-3.5 w-3.5 text-muted-foreground shrink-0" />
                        {tool.name}
                      </CardTitle>
                      {tool.description && (
                        <CardDescription className="text-xs">{tool.description}</CardDescription>
                      )}
                    </CardHeader>
                    {(tool.input_schema || tool.annotations) && (
                      <CardContent className="pb-3 px-4 space-y-2">
                        {tool.annotations && Object.keys(tool.annotations).length > 0 && (
                          <div className="flex flex-wrap gap-1">
                            {Object.entries(tool.annotations).map(([k, v]) =>
                              typeof v === 'boolean' && v ? (
                                <Badge key={k} variant="secondary" className="text-[10px] px-1.5 py-0">
                                  {k}
                                </Badge>
                              ) : null,
                            )}
                          </div>
                        )}
                        {tool.input_schema && (
                          <RawJsonViewer
                            data={tool.input_schema}
                            title="Input schema"
                          />
                        )}
                      </CardContent>
                    )}
                  </Card>
                ))}
              </div>
            ) : (
              <EmptyState
                icon={<Cpu className="h-8 w-8 text-muted-foreground" />}
                title="No tools declared"
                description="This server has not declared any tools. MCP clients can still query the server's runtime tools/list method if the server advertises the tools capability."
              />
            )}
          </TabsContent>

          {/* ── Versions Tab ── */}
          <TabsContent value="versions" className="mt-6 space-y-4">
            <VersionHistory
              type="mcp"
              namespace={data.namespace}
              slug={data.slug}
              latestVersion={lv?.version}
            />
          </TabsContent>

          {/* ── JSON Tab ── */}
          <TabsContent value="json" className="mt-6">
            <RawJsonViewer data={data} title="Raw API response" defaultOpen />
          </TabsContent>
        </Tabs>

        <Separator />
        <RelatedEntries type="mcp" namespace={data.namespace} currentSlug={data.slug} />
      </main>
      <Footer />
    </div>
  )
}
