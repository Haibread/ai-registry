import { useRef } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link, useParams, useLocation, useNavigate } from 'react-router-dom'
import {
  ExternalLink,
  Cpu,
  Shield,
  AlertTriangle,
  FileText,
  Link2,
  GitBranch,
  CalendarClock,
  Building2,
  Activity,
  ArrowDownToLine,
  ArrowUpFromLine,
  Package2,
} from 'lucide-react'
import { Header } from '@/components/layout/header'
import { Footer } from '@/components/layout/footer'
import { Badge, StatusBadge, VisibilityBadge, VerifiedBadge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Separator } from '@/components/ui/separator'
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs'
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '@/components/ui/card'
import { RawJsonViewer } from '@/components/ui/raw-json-viewer'
import { Breadcrumbs } from '@/components/ui/breadcrumbs'
import { DetailPageSkeleton } from '@/components/ui/detail-page-skeleton'
import { EmptyState } from '@/components/ui/empty-state'
import { TooltipInfo } from '@/components/ui/tooltip-info'
import { CopyButton } from '@/components/ui/copy-button'
import { ResourceIcon } from '@/components/ui/resource-icon'
import { FreshnessIndicator } from '@/components/ui/freshness-indicator'
import { AuthGuide } from '@/components/agents/auth-guide'
import { AgentSnippetGenerator } from '@/components/agents/snippet-generator'
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
import { formatDate } from '@/lib/utils'
import { getFieldExplanation } from '@/lib/field-explanations'
import { getModeLabel, getModeInfo } from '@/lib/mode-labels'
import type { components } from '@/lib/schema'

type AgentSkill = components['schemas']['AgentSkill']

export default function AgentDetailPage() {
  const { ns, slug } = useParams<{ ns: string; slug: string }>()
  const location = useLocation()
  const navigate = useNavigate()
  const api = getPublicClient()
  const { data, isLoading, isError } = useQuery({
    queryKey: ['agent', ns, slug],
    queryFn: () => api.GET('/api/v1/agents/{namespace}/{slug}', {
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
  useRecordView('agent', data?.namespace, data?.slug)
  const recordCopy = useRecordCopy('agent', data?.namespace, data?.slug)

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
          icon={<ResourceIcon type="agent" className="h-10 w-10" />}
          title="Agent not found"
          description="The agent you're looking for doesn't exist or has been removed."
          action={<Button variant="outline" size="sm" asChild><Link to="/agents">Back to Agents</Link></Button>}
        />
      </main>
      <Footer />
    </div>
  )

  const lv = data.latest_version
  const cardUrl = `/agents/${ns}/${slug}/.well-known/agent-card.json`

  // Extract extra fields from the latest version (may not be in the typed schema for list responses)
  const lvAny = lv as Record<string, unknown> | undefined
  const iconUrl = lvAny?.icon_url as string | undefined
  const documentationUrl = lvAny?.documentation_url as string | undefined
  const provider = lvAny?.provider as Record<string, unknown> | undefined
  const statusMessage = lvAny?.status_message as string | undefined

  return (
    <div className="flex min-h-screen flex-col">
      <Header />
      <StickyDetailHeader
        type="agent"
        name={data.name}
        version={lv?.version}
        identifier={`${data.namespace}/${data.slug}`}
        titleRef={titleRef}
      />
      <main className="flex-1 container py-8 space-y-6">
        <Breadcrumbs
          segments={[
            { label: 'Home', href: '/' },
            { label: 'Agents', href: '/agents' },
            { label: data.namespace, href: `/agents/${data.namespace}` },
            { label: data.slug },
          ]}
        />

        {/* Status message banner */}
        {statusMessage && (
          <div className="flex items-center gap-2 rounded-md border border-yellow-500/30 bg-yellow-500/10 px-4 py-3 text-sm text-yellow-800 dark:text-yellow-200 max-w-prose">
            <AlertTriangle className="h-4 w-4 shrink-0" />
            {statusMessage}
          </div>
        )}

        {/* Title row */}
        <div className="space-y-2">
          <div className="flex items-center gap-3 flex-wrap">
            {iconUrl && (
              <img src={iconUrl} alt="" className="h-10 w-10 rounded-lg shrink-0 object-cover" />
            )}
            <h1 ref={titleRef} className="text-2xl sm:text-3xl font-bold min-w-0 break-words">{data.name}</h1>
            <div className="flex items-center gap-2 flex-wrap">
              {lv && (
                <Badge variant="outline" className="font-mono">v{lv.version}</Badge>
              )}
              {data.verified && <VerifiedBadge />}
              <StatusBadge status={data.status} />
              <VisibilityBadge visibility={data.visibility} />
            </div>
          </div>
          <div className="flex items-center gap-2 flex-wrap">
            <p className="text-sm text-muted-foreground font-mono">
              <Link to={`/agents/${data.namespace}`} className="hover:text-foreground transition-colors">
                {data.namespace}
              </Link>
              /{data.slug}
            </p>
            <CopyButton value={`${data.namespace}/${data.slug}`} label="Copy identifier" />
            <span className="h-4 w-px bg-border mx-1" aria-hidden="true" />
            <Button variant="outline" size="sm" asChild>
              <a href={cardUrl} target="_blank" rel="noopener noreferrer" className="flex items-center gap-1.5">
                <ExternalLink className="h-4 w-4" /> A2A Agent Card
              </a>
            </Button>
            {documentationUrl && (
              <Button variant="outline" size="sm" asChild>
                <a href={documentationUrl} target="_blank" rel="noopener noreferrer" className="flex items-center gap-1.5">
                  <FileText className="h-4 w-4" /> Documentation
                </a>
              </Button>
            )}
            <ReportDialog
              resourceType="agent"
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
            <TabsTrigger value="skills">
              Skills{lv?.skills && lv.skills.length > 0 ? ` (${lv.skills.length})` : ''}
            </TabsTrigger>
            <TabsTrigger value="connect">Connect</TabsTrigger>
            <TabsTrigger value="versions">Versions</TabsTrigger>
            <TabsTrigger value="json">JSON</TabsTrigger>
          </TabsList>

          {/* ── Overview Tab ── */}
          {/* mt-6 overrides the TabsContent default mt-2 so the gap from the
              tabs to the first child matches the rhythm below. */}
          <TabsContent value="overview" className="mt-6 space-y-8">
            {/* Publisher banner */}
            <PublisherSidebar namespace={data.namespace} />

            {/* ─── Connection ───
                Everything a caller needs to invoke this agent: endpoint URL,
                spec version, supported IO modes, and authentication schemes.
                Grouped in one card so users see "how do I talk to this" at a
                glance. */}
            <section className="space-y-3">
              <SectionHeader icon={<Link2 />} title="Connection" />
              <div className="rounded-xl border bg-card overflow-hidden shadow-xs">
                {/* Endpoint URL — hero row, full width */}
                {lv?.endpoint_url && (
                  <StatTile
                    className="px-5 py-4"
                    label="Endpoint URL"
                    icon={<Link2 />}
                    tooltip={getFieldExplanation('endpoint_url') ?? undefined}
                  >
                    <div className="flex items-center gap-2 min-w-0">
                      <a
                        href={lv.endpoint_url}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="font-mono text-xs hover:underline truncate"
                      >
                        {lv.endpoint_url}
                      </a>
                      <CopyButton value={lv.endpoint_url} label="Copy endpoint URL" onCopy={recordCopy} />
                    </div>
                  </StatTile>
                )}
                {/* Protocol / Input / Output / Auth — 4-col grid below */}
                <div className="flex flex-col sm:flex-row sm:divide-x divide-y sm:divide-y-0 border-t">
                  <StatTile
                    className="flex-1 px-5 py-4"
                    label="A2A Protocol"
                    icon={<GitBranch />}
                    tooltip={getFieldExplanation('a2a_protocol_version') ?? undefined}
                  >
                    {lv?.protocol_version ? (
                      <span className="font-mono">{lv.protocol_version}</span>
                    ) : (
                      <span className="text-muted-foreground">—</span>
                    )}
                  </StatTile>
                  <StatTile
                    className="flex-1 px-5 py-4"
                    label="Input modes"
                    icon={<ArrowDownToLine />}
                  >
                    {lv?.default_input_modes && lv.default_input_modes.length > 0 ? (
                      <div className="flex flex-wrap gap-1">
                        {lv.default_input_modes.map((m) => {
                          const info = getModeInfo(m)
                          return (
                            <span key={m} className="flex items-center gap-1">
                              <Badge variant="secondary" className="text-xs">{getModeLabel(m)}</Badge>
                              {info && <TooltipInfo content={info.description} />}
                            </span>
                          )
                        })}
                      </div>
                    ) : (
                      <span className="text-muted-foreground">—</span>
                    )}
                  </StatTile>
                  <StatTile
                    className="flex-1 px-5 py-4"
                    label="Output modes"
                    icon={<ArrowUpFromLine />}
                  >
                    {lv?.default_output_modes && lv.default_output_modes.length > 0 ? (
                      <div className="flex flex-wrap gap-1">
                        {lv.default_output_modes.map((m) => {
                          const info = getModeInfo(m)
                          return (
                            <span key={m} className="flex items-center gap-1">
                              <Badge variant="secondary" className="text-xs">{getModeLabel(m)}</Badge>
                              {info && <TooltipInfo content={info.description} />}
                            </span>
                          )
                        })}
                      </div>
                    ) : (
                      <span className="text-muted-foreground">—</span>
                    )}
                  </StatTile>
                  <StatTile
                    className="flex-1 px-5 py-4"
                    label="Authentication"
                    icon={<Shield />}
                  >
                    {lv?.authentication && lv.authentication.length > 0 ? (
                      <div className="flex flex-wrap gap-1">
                        {lv.authentication.map((scheme, i) => {
                          const s = scheme as Record<string, string>
                          const label = s['scheme'] ?? s['type'] ?? `scheme ${i + 1}`
                          return (
                            <span key={i} className="flex items-center gap-1">
                              <Badge variant="outline" className="text-xs">{label}</Badge>
                              {getFieldExplanation(label) && (
                                <TooltipInfo content={getFieldExplanation(label)!} />
                              )}
                            </span>
                          )
                        })}
                      </div>
                    ) : (
                      <span className="text-muted-foreground">Public</span>
                    )}
                  </StatTile>
                </div>
              </div>
            </section>

            {/* ─── Release ───
                Version-level facts — who published what, when, at what status. */}
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
                    label="Provider"
                    icon={<Building2 />}
                  >
                    {provider && typeof provider.organization === 'string' ? (
                      provider.organization
                    ) : (
                      <span className="text-muted-foreground">—</span>
                    )}
                  </StatTile>
                  <StatTile
                    className="flex-1 px-5 py-4"
                    label="Status"
                    icon={<Activity />}
                  >
                    <Badge variant="secondary" className="capitalize">{data.status}</Badge>
                  </StatTile>
                </div>
              </div>
            </section>

            {/* ─── Engagement ───
                De-emphasized engagement footnote. Kept as a compact inline
                strip because skills and capabilities matter more than vanity
                counts on this page. */}
            <EngagementStrip
              viewCount={data.view_count ?? 0}
              copyCount={data.copy_count ?? 0}
              createdAt={data.created_at}
              updatedAt={data.updated_at}
            />

            {/* ─── Activity ───
                Privacy-scrubbed lifecycle feed of this agent's mutations. */}
            <ActivityFeed
              resourceType="agent"
              namespace={ns}
              slug={slug}
            />
          </TabsContent>

          {/* ── Skills Tab ── */}
          <TabsContent value="skills" className="mt-6 space-y-4">
            {lv?.skills && lv.skills.length > 0 ? (
              <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
                {lv.skills.map((skill: AgentSkill) => (
                  <Card key={skill.id} className="bg-muted/30">
                    <CardHeader className="pb-2 pt-4 px-4">
                      <CardTitle className="text-sm flex items-center gap-2">
                        <Cpu className="h-3.5 w-3.5 text-muted-foreground shrink-0" />
                        {skill.name}
                      </CardTitle>
                      <CardDescription className="text-xs">{skill.description}</CardDescription>
                    </CardHeader>
                    {(skill.tags.length > 0 || (skill.examples && skill.examples.length > 0)) && (
                      <CardContent className="pb-3 px-4 space-y-2">
                        {skill.tags.length > 0 && (
                          <div className="flex flex-wrap gap-1">
                            {skill.tags.map((tag) => (
                              <Badge key={tag} variant="secondary" className="text-[10px] px-1.5 py-0">
                                {tag}
                              </Badge>
                            ))}
                          </div>
                        )}
                        {skill.examples && skill.examples.length > 0 && (
                          <div className="space-y-1">
                            <p className="text-[10px] text-muted-foreground uppercase tracking-wide">Examples</p>
                            <ul className="text-xs space-y-0.5 text-muted-foreground">
                              {skill.examples.slice(0, 3).map((ex, i) => (
                                <li key={i} className="truncate">&bull; {ex}</li>
                              ))}
                            </ul>
                          </div>
                        )}
                      </CardContent>
                    )}
                  </Card>
                ))}
              </div>
            ) : (
              <EmptyState
                icon={<Cpu className="h-8 w-8 text-muted-foreground" />}
                title="No skills defined"
                description="This agent has not declared any skills yet."
              />
            )}
          </TabsContent>

          {/* ── Connect Tab ── */}
          <TabsContent value="connect" className="mt-6 space-y-6 max-w-3xl">
            {lv?.authentication && lv.authentication.length > 0 && (
              <AuthGuide schemes={lv.authentication as Array<Record<string, string>>} />
            )}
            {lv?.endpoint_url && (
              <AgentSnippetGenerator
                endpointUrl={lv.endpoint_url}
                authSchemes={
                  lv.authentication
                    ? (lv.authentication as Array<Record<string, string>>).map(
                        (s) => s.scheme ?? s.type ?? '',
                      )
                    : undefined
                }
              />
            )}
            {!lv?.endpoint_url && (
              <EmptyState
                icon={<Cpu className="h-8 w-8 text-muted-foreground" />}
                title="No endpoint available"
                description="This agent has not published an endpoint URL yet."
              />
            )}
          </TabsContent>

          {/* ── Versions Tab ── */}
          <TabsContent value="versions" className="mt-6 space-y-4">
            <VersionHistory
              type="agent"
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
        <RelatedEntries type="agent" namespace={data.namespace} currentSlug={data.slug} />
      </main>
      <Footer />
    </div>
  )
}
