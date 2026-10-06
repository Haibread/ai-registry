import { useRef } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link, useParams, useLocation, useNavigate } from 'react-router-dom'
import { AlertTriangle, Cpu, ExternalLink, FileText, Shield } from 'lucide-react'
import { Header } from '@/components/layout/header'
import { Footer } from '@/components/layout/footer'
import { Badge, FeaturedBadge, StatusBadge, VisibilityBadge, VerifiedBadge } from '@/components/ui/badge'
import { TagBadge } from '@/components/ui/tag-badge'
import { indexInstanceTags, useInstanceTags } from '@/lib/use-instance-tags'
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
import { DetailFacts, type DetailFact } from '@/components/shared/detail-facts'
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
  const hashTab = location.hash?.replace('#', '')
  // Shared links still carry #connect, the Usage tab's former name.
  const activeTab = (hashTab === 'connect' ? 'usage' : hashTab) || 'overview'
  const handleTabChange = (value: string) => {
    navigate(`${location.pathname}#${value}`, { replace: true })
  }

  // Hooks must run unconditionally (Rules of Hooks): declare refs and
  // tracking hooks before any early returns so hook order is stable across
  // loading → loaded transitions.
  const titleRef = useRef<HTMLHeadingElement>(null)
  useRecordView('agent', data?.namespace, data?.slug)
  const recordCopy = useRecordCopy('agent', data?.namespace, data?.slug)
  const { data: tagData } = useInstanceTags()
  const tagIndex = indexInstanceTags(tagData?.items)

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

  const authSchemes = (lv?.authentication ?? []).map((scheme, i) => {
    const s = scheme as Record<string, string>
    return s['scheme'] ?? s['type'] ?? `scheme ${i + 1}`
  })
  const tags = data.tags ?? []

  const modeBadges = (modes: string[] | undefined) =>
    modes && modes.length > 0 ? (
      <span className="flex flex-wrap gap-1">
        {modes.map((m) => {
          const info = getModeInfo(m)
          return (
            <span key={m} className="flex items-center gap-1">
              <Badge variant="secondary" className="rounded-md">{getModeLabel(m)}</Badge>
              {info && <TooltipInfo content={info.description} />}
            </span>
          )
        })}
      </span>
    ) : <span className="text-muted-foreground">—</span>

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
    {
      label: 'A2A protocol',
      tooltip: getFieldExplanation('a2a_protocol_version'),
      value: lv?.protocol_version
        ? <span className="font-mono">{lv.protocol_version}</span>
        : <span className="text-muted-foreground">—</span>,
    },
    { label: 'Input', value: modeBadges(lv?.default_input_modes) },
    { label: 'Output', value: modeBadges(lv?.default_output_modes) },
    ...(provider && typeof provider.organization === 'string'
      ? [{ label: 'Provider', value: provider.organization }]
      : []),
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

        {statusMessage && (
          <div className="flex max-w-prose items-center gap-2 rounded-md border border-warning/40 bg-warning/10 px-4 py-3 text-sm">
            <AlertTriangle className="h-4 w-4 shrink-0 text-warning" aria-hidden="true" />
            {statusMessage}
          </div>
        )}

        <div className="flex flex-col gap-4 sm:flex-row sm:items-start">
          {iconUrl ? (
            <img src={iconUrl} alt="" className="h-14 w-14 shrink-0 rounded-2xl object-cover" />
          ) : (
            <div
              className="flex h-14 w-14 shrink-0 items-center justify-center rounded-2xl bg-secondary text-secondary-foreground"
              aria-hidden="true"
            >
              <ResourceIcon type="agent" className="h-7 w-7" />
            </div>
          )}
          <div className="min-w-0 flex-1 space-y-1.5">
            <div className="flex flex-wrap items-center gap-2">
              <h1 ref={titleRef} className="min-w-0 break-words text-2xl font-bold tracking-tight sm:text-3xl">{data.name}</h1>
              {data.verified && <VerifiedBadge />}
              {data.featured && <FeaturedBadge />}
              {data.status !== 'published' && <StatusBadge status={data.status} />}
              {data.visibility === 'private' && <VisibilityBadge visibility={data.visibility} />}
            </div>
            <div className="flex items-center gap-1 font-mono text-sm text-muted-foreground">
              <Link to={`/agents/${data.namespace}`} className="hover:text-foreground transition-colors">
                {data.namespace}
              </Link>
              /{data.slug}
              <CopyButton value={`${data.namespace}/${data.slug}`} label="Copy identifier" />
            </div>
            {data.description && <p className="max-w-prose pt-1 text-muted-foreground">{data.description}</p>}
          </div>
          {documentationUrl && (
            <Button variant="outline" size="sm" asChild className="self-start">
              <a href={documentationUrl} target="_blank" rel="noopener noreferrer">
                <FileText className="h-4 w-4" aria-hidden="true" /> Documentation
              </a>
            </Button>
          )}
        </div>

        <div className="grid gap-8 lg:grid-cols-[minmax(0,1fr)_20rem]">
          {/* The side column comes first in the DOM so a phone shows how to
              connect before the README; on large screens it moves right. */}
          <aside className="space-y-4 lg:order-2" aria-label="Agent facts">
            <section className="space-y-3 rounded-xl border bg-card p-4 shadow-xs" aria-label="Quick connect">
              <h2 className="text-sm font-semibold">Quick connect</h2>
              {lv?.endpoint_url ? (
                <div className="flex items-center gap-1 rounded-md border bg-muted/50 py-1 pl-3 pr-1">
                  <span className="min-w-0 flex-1 truncate font-mono text-xs" title={lv.endpoint_url}>{lv.endpoint_url}</span>
                  <CopyButton value={lv.endpoint_url} label="Copy endpoint URL" onCopy={recordCopy} />
                </div>
              ) : (
                <p className="text-sm text-muted-foreground">No endpoint published yet.</p>
              )}
              <div className="flex flex-wrap items-center gap-1.5 text-xs text-muted-foreground">
                <Shield className="h-3 w-3" aria-hidden="true" />
                {authSchemes.length > 0
                  ? authSchemes.map((label) => (
                      <span key={label} className="flex items-center gap-1">
                        <Badge variant="outline" className="rounded-md">{label}</Badge>
                        {getFieldExplanation(label) && <TooltipInfo content={getFieldExplanation(label)!} />}
                      </span>
                    ))
                  : 'No authentication'}
              </div>
              <Button className="w-full" onClick={() => handleTabChange('usage')}>
                Set up a client
              </Button>
              <a
                href={cardUrl}
                target="_blank"
                rel="noopener noreferrer"
                className="inline-flex items-center gap-1 text-sm text-primary hover:underline"
              >
                <ExternalLink className="h-3.5 w-3.5" aria-hidden="true" /> A2A Agent Card
              </a>
            </section>

            <DetailFacts facts={facts} />
            <PublisherSidebar namespace={data.namespace} />

            <nav aria-label="Agent links" className="flex flex-wrap items-center gap-x-4 gap-y-1 px-1 text-sm">
              <ReportDialog
                resourceType="agent"
                resourceId={data.id}
                resourceLabel={`${data.namespace}/${data.slug}`}
              />
            </nav>
          </aside>

          <div className="min-w-0">
            <Tabs value={activeTab} onValueChange={handleTabChange}>
              <TabsList>
                <TabsTrigger value="overview">Overview</TabsTrigger>
                <TabsTrigger value="skills">
                  Skills{lv?.skills && lv.skills.length > 0 ? ` (${lv.skills.length})` : ''}
                </TabsTrigger>
                <TabsTrigger value="usage">Usage</TabsTrigger>
                <TabsTrigger value="versions">Versions</TabsTrigger>
              </TabsList>

              {/* mt-6 overrides the TabsContent default mt-2 so every tab
                  starts at the same distance from the tab bar. */}
              <TabsContent value="overview" className="mt-6 space-y-8">
                {data.readme ? (
                  <MarkdownRenderer content={data.readme} />
                ) : (
                  <p className="text-sm text-muted-foreground">The publisher has not written a README for this agent.</p>
                )}
                <ActivityFeed resourceType="agent" namespace={ns} slug={slug} />
                <RawJsonViewer data={data} title="Raw API response" />
              </TabsContent>

              <TabsContent value="skills" className="mt-6 space-y-4">
                {lv?.skills && lv.skills.length > 0 ? (
                  <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
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

              <TabsContent value="usage" className="mt-6 space-y-6">
                {lv?.authentication && lv.authentication.length > 0 && (
                  <AuthGuide schemes={lv.authentication as Array<Record<string, string>>} />
                )}
                {lv?.endpoint_url ? (
                  <AgentSnippetGenerator endpointUrl={lv.endpoint_url} authSchemes={authSchemes} />
                ) : (
                  <EmptyState
                    icon={<Cpu className="h-8 w-8 text-muted-foreground" />}
                    title="No endpoint available"
                    description="This agent has not published an endpoint URL yet."
                  />
                )}
              </TabsContent>

              <TabsContent value="versions" className="mt-6 space-y-4">
                <VersionHistory
                  type="agent"
                  namespace={data.namespace}
                  slug={data.slug}
                  latestVersion={lv?.version}
                />
              </TabsContent>
            </Tabs>
          </div>
        </div>

        <Separator />
        <RelatedEntries type="agent" namespace={data.namespace} currentSlug={data.slug} />
      </main>
      <Footer />
    </div>
  )
}
