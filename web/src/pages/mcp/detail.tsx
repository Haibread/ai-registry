import { useRef } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link, useParams, useLocation, useNavigate } from 'react-router-dom'
import { Code2, Cpu, ExternalLink, EyeOff, GitFork, Shield } from 'lucide-react'
import { Header } from '@/components/layout/header'
import { Footer } from '@/components/layout/footer'
import { Badge, FeaturedBadge, StatusBadge, VisibilityBadge, VerifiedBadge } from '@/components/ui/badge'
import { TagBadge } from '@/components/ui/tag-badge'
import { indexInstanceTags, useInstanceTags } from '@/lib/use-instance-tags'
import { Button } from '@/components/ui/button'
import { Separator } from '@/components/ui/separator'
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs'
import { RawJsonViewer } from '@/components/ui/raw-json-viewer'
import { Breadcrumbs } from '@/components/ui/breadcrumbs'
import { DetailPageSkeleton } from '@/components/ui/detail-page-skeleton'
import { EmptyState } from '@/components/ui/empty-state'
import { TooltipInfo } from '@/components/ui/tooltip-info'
import { CopyButton } from '@/components/ui/copy-button'
import { ResourceIcon } from '@/components/ui/resource-icon'
import { FreshnessIndicator } from '@/components/ui/freshness-indicator'
import { CapabilitiesSection } from '@/components/mcp/capabilities-section'
import { ToolsExplorer } from '@/components/mcp/tools-explorer'
import { UsageTab } from '@/components/mcp/usage-tab'
import { MarkdownRenderer } from '@/components/ui/markdown-renderer'
import { PublisherSidebar } from '@/components/shared/publisher-sidebar'
import { SectionHeader } from '@/components/shared/section-header'
import { DetailFacts, type DetailFact } from '@/components/shared/detail-facts'
import { ActivityFeed } from '@/components/shared/activity-feed'
import { RelatedEntries } from '@/components/shared/related-entries'
import { VersionHistory } from '@/components/shared/version-history'
import { StickyDetailHeader } from '@/components/shared/sticky-detail-header'
import { ReportDialog } from '@/components/shared/report-dialog'
import { useRecordView, useRecordCopy } from '@/hooks/use-record-event'
import { useCatalogClient } from '@/lib/api-client'
import { formatDate, getInstallCommand, isRemoteTransport } from '@/lib/utils'
import { getFieldExplanation } from '@/lib/field-explanations'
import { ProtocolVersionBadges } from '@/components/mcp/protocol-version-badges'

export default function MCPDetailPage() {
  const { ns, slug } = useParams<{ ns: string; slug: string }>()
  const location = useLocation()
  const navigate = useNavigate()
  const { api, viewer, ready } = useCatalogClient()
  const { data, isPending, isError } = useQuery({
    queryKey: ['mcp-server', ns, slug, viewer],
    queryFn: () => api.GET('/api/v1/mcp/servers/{namespace}/{slug}', {
      params: { path: { namespace: ns!, slug: slug! } },
    }).then(r => r.data),
    enabled: ready && !!ns && !!slug,
  })
  // latest_version only ever holds a published version. A member looking at
  // an entry that has none yet gets its newest version instead, so the page
  // still shows what is being prepared.
  const { data: unpublishedVersion } = useQuery({
    queryKey: ['mcp-server-unpublished-version', ns, slug, viewer],
    queryFn: () => api.GET('/api/v1/mcp/servers/{namespace}/{slug}/versions', {
      params: { path: { namespace: ns!, slug: slug! } },
    }).then(r => r.data?.items[0] ?? null),
    enabled: !!data && !data.latest_version,
  })

  // Tab state synced to URL hash
  const hashTab = location.hash?.replace('#', '')
  // Shared links still carry #installation, the Usage tab's former name.
  const activeTab = (hashTab === 'installation' ? 'usage' : hashTab) || 'overview'
  const handleTabChange = (value: string) => {
    navigate(`${location.pathname}#${value}`, { replace: true })
  }

  // Hooks must run unconditionally (Rules of Hooks): declare refs and
  // tracking hooks before any early returns so hook order is stable across
  // loading → loaded transitions.
  const titleRef = useRef<HTMLHeadingElement>(null)
  // Only the public catalog counts views; a member previewing a private entry
  // is not an audience.
  useRecordView(data?.visibility === 'public' ? data.namespace : undefined, data?.slug)
  const recordCopy = useRecordCopy(data?.namespace, data?.slug)
  const { data: tagData } = useInstanceTags()
  const tagIndex = indexInstanceTags(tagData?.items)

  if (isPending && !isError) return (
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

  const lv = data.latest_version ?? unpublishedVersion ?? undefined
  const isPreview = data.visibility === 'private' || !data.latest_version
  const capabilities = (lv as Record<string, unknown> | undefined)?.capabilities as Record<string, unknown> | undefined
  // Connection targets for hosted servers: first-class remote endpoints
  // (lv.remotes) plus any remote-transport package that carries a URL.
  const remoteEndpoints = [
    ...(lv?.remotes ?? []).map((r) => ({ type: r.type, url: r.url })),
    ...(lv?.packages ?? [])
      .filter((p) => isRemoteTransport(p.transport.type) && !!p.transport.url)
      .map((p) => ({ type: p.transport.type, url: p.transport.url! })),
  ]
  const localPackage = remoteEndpoints.length === 0 ? lv?.packages?.[0] : undefined
  const tags = data.tags ?? []

  const facts: DetailFact[] = [
    {
      label: 'Version',
      value: lv ? (
        <span className="flex flex-wrap items-center gap-x-2">
          <span className="font-mono">v{lv.version}</span>
          <span className="text-muted-foreground">
            {lv.published_at ? formatDate(lv.published_at) : 'Draft'}
          </span>
        </span>
      ) : '—',
    },
    ...(lv ? [
      {
        label: remoteEndpoints.length > 0 ? 'Transport' : 'Runtime',
        tooltip: getFieldExplanation(lv.runtime) ?? getFieldExplanation('runtime'),
        value: <Badge variant="secondary" className="rounded-md">{lv.runtime}</Badge>,
      },
      {
        label: 'Protocol',
        tooltip: getFieldExplanation('protocol_versions'),
        value: <ProtocolVersionBadges versions={lv.protocol_versions} />,
      },
    ] : []),
    { label: 'License', value: data.license || <span className="text-muted-foreground">—</span> },
    ...(tags.length > 0 ? [{
      label: 'Tags',
      value: (
        <span className="flex flex-wrap gap-1">
          {tags.map((t) => <TagBadge key={t} slug={t} tag={tagIndex.get(t)} />)}
        </span>
      ),
    }] : []),
    {
      label: 'Usage',
      value: `${(data.view_count ?? 0).toLocaleString()} views · ${(data.copy_count ?? 0).toLocaleString()} installs`,
    },
    { label: 'Updated', value: <FreshnessIndicator updatedAt={data.updated_at} className="text-sm" /> },
  ]

  return (
    <div className="flex min-h-screen flex-col">
      <Header />
      <StickyDetailHeader
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

        {isPreview && (
          <div
            role="status"
            className="flex flex-wrap items-center gap-x-3 gap-y-2 rounded-md border border-warning/40 bg-warning/10 px-3 py-2 text-sm"
          >
            <EyeOff className="h-4 w-4 shrink-0" aria-hidden="true" />
            <span className="flex-1 min-w-0">
              {data.visibility === 'private'
                ? <>Only members of <span className="font-mono">{data.namespace}</span> can see this server.</>
                : 'This server is not in the public catalog.'}
              {!data.latest_version && lv && (
                <> Showing unpublished version <span className="font-mono">v{lv.version}</span>.</>
              )}
            </span>
            <Button variant="outline" size="sm" asChild>
              <Link to={`/admin/mcp/${data.namespace}/${data.slug}`}>Manage</Link>
            </Button>
          </div>
        )}

        <div className="flex flex-col gap-4 sm:flex-row sm:items-start">
          <div
            className="flex h-14 w-14 shrink-0 items-center justify-center rounded-2xl bg-secondary text-secondary-foreground"
            aria-hidden="true"
          >
            <ResourceIcon type="mcp-server" className="h-7 w-7" />
          </div>
          <div className="min-w-0 flex-1 space-y-1.5">
            <div className="flex flex-wrap items-center gap-2">
              <h1 ref={titleRef} className="min-w-0 break-words text-2xl font-bold tracking-tight sm:text-3xl">{data.name}</h1>
              {data.verified && <VerifiedBadge />}
              {data.featured && <FeaturedBadge />}
              {data.status !== 'published' && <StatusBadge status={data.status} />}
              {data.visibility === 'private' && <VisibilityBadge visibility={data.visibility} />}
            </div>
            <div className="flex items-center gap-1 font-mono text-sm text-muted-foreground">
              <Link to={`/mcp/${data.namespace}`} className="hover:text-foreground transition-colors">
                {data.namespace}
              </Link>
              /{data.slug}
              <CopyButton value={`${data.namespace}/${data.slug}`} label="Copy identifier" />
            </div>
            {data.description && <p className="max-w-prose pt-1 text-muted-foreground">{data.description}</p>}
          </div>
          {data.repo_url && (
            <Button variant="outline" size="sm" asChild className="self-start">
              <a href={data.repo_url} target="_blank" rel="noopener noreferrer">
                <GitFork className="h-4 w-4" aria-hidden="true" /> Repository
              </a>
            </Button>
          )}
        </div>

        <div className="grid gap-8 lg:grid-cols-[minmax(0,1fr)_20rem]">
          {/* The side column comes first in the DOM so a phone shows how to
              connect before the README; on large screens it moves right. */}
          <aside className="space-y-4 lg:order-2" aria-label="Server facts">
            <section className="space-y-3 rounded-xl border bg-card p-4 shadow-xs" aria-label="Quick connect">
              <h2 className="text-sm font-semibold">Quick connect</h2>
              {remoteEndpoints.map((ep) => (
                <div key={ep.url} className="space-y-1.5">
                  <div className="flex flex-wrap items-center gap-2 text-xs">
                    <Badge variant="secondary" className="rounded-md">{ep.type}</Badge>
                    <span className="inline-flex items-center gap-1 text-muted-foreground">
                      <Shield className="h-3 w-3" aria-hidden="true" /> Auth per MCP spec (OAuth 2.1)
                      {getFieldExplanation('mcp_authentication') && (
                        <TooltipInfo content={getFieldExplanation('mcp_authentication')!} />
                      )}
                    </span>
                  </div>
                  <div className="flex items-center gap-1 rounded-md border bg-muted/50 py-1 pl-3 pr-1">
                    <span className="min-w-0 flex-1 truncate font-mono text-xs" title={ep.url}>{ep.url}</span>
                    <CopyButton value={ep.url} label="Copy endpoint URL" onCopy={recordCopy} />
                  </div>
                </div>
              ))}
              {localPackage && (
                <div className="space-y-1.5">
                  <p className="text-xs text-muted-foreground">Runs locally over {lv?.runtime}</p>
                  <div className="flex items-center gap-1 rounded-md border bg-muted/50 py-1 pl-3 pr-1">
                    <span className="min-w-0 flex-1 truncate font-mono text-xs">{getInstallCommand(localPackage)}</span>
                    <CopyButton value={getInstallCommand(localPackage)} label="Copy run command" onCopy={recordCopy} />
                  </div>
                </div>
              )}
              <Button className="w-full" onClick={() => handleTabChange('usage')}>
                Set up in your client
              </Button>
            </section>

            <DetailFacts facts={facts} />
            <PublisherSidebar namespace={data.namespace} />

            <nav aria-label="Server links" className="flex flex-wrap items-center gap-x-4 gap-y-1 px-1 text-sm">
              {data.homepage_url && (
                <a href={data.homepage_url} target="_blank" rel="noopener noreferrer" className="inline-flex items-center gap-1 text-primary hover:underline">
                  <ExternalLink className="h-3.5 w-3.5" aria-hidden="true" /> Homepage
                </a>
              )}
              <ReportDialog
                resourceId={data.id}
                resourceLabel={`${data.namespace}/${data.slug}`}
              />
            </nav>
          </aside>

          <div className="min-w-0">
            <Tabs value={activeTab} onValueChange={handleTabChange}>
              <TabsList>
                <TabsTrigger value="overview">Overview</TabsTrigger>
                <TabsTrigger value="usage">Usage</TabsTrigger>
                <TabsTrigger value="tools">
                  Tools{lv?.tools && lv.tools.length > 0 ? ` (${lv.tools.length})` : ''}
                </TabsTrigger>
                <TabsTrigger value="versions">Versions</TabsTrigger>
              </TabsList>

              {/* mt-6 overrides the TabsContent default mt-2 so every tab
                  starts at the same distance from the tab bar. */}
              <TabsContent value="overview" className="mt-6 space-y-8">
                {data.readme ? (
                  <MarkdownRenderer content={data.readme} />
                ) : (
                  <p className="text-sm text-muted-foreground">The publisher has not written a README for this server.</p>
                )}

                {capabilities && Object.keys(capabilities).length > 0 && (
                  <section className="space-y-3">
                    <SectionHeader icon={<Code2 />} title="Capabilities" />
                    <CapabilitiesSection capabilities={capabilities} hideTitle />
                  </section>
                )}

                <ActivityFeed namespace={ns} slug={slug} />

                <RawJsonViewer data={data} title="Raw API response" />
              </TabsContent>

              <TabsContent value="usage" className="mt-6 space-y-6">
                <UsageTab server={data} version={lv} onCopy={recordCopy} />
              </TabsContent>

              {/* The publisher-declared `tools[]` array, not the
                  `capabilities.tools` negotiation flag ({listChanged}). */}
              <TabsContent value="tools" className="mt-6 space-y-4">
                {lv?.tools && lv.tools.length > 0 ? (
                  <ToolsExplorer tools={lv.tools} />
                ) : (
                  <EmptyState
                    icon={<Cpu className="h-8 w-8 text-muted-foreground" />}
                    title="No tools declared"
                    description="This server has not declared any tools. MCP clients can still query the server's runtime tools/list method if the server advertises the tools capability."
                  />
                )}
              </TabsContent>

              <TabsContent value="versions" className="mt-6 space-y-4">
                <VersionHistory
                  namespace={data.namespace}
                  slug={data.slug}
                  latestVersion={lv?.version}
                />
              </TabsContent>
            </Tabs>
          </div>
        </div>

        <Separator />
        <RelatedEntries namespace={data.namespace} currentSlug={data.slug} />
      </main>
      <Footer />
    </div>
  )
}
