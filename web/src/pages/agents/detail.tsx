import { useQuery } from '@tanstack/react-query'
import { Link, useParams, useLocation, useNavigate } from 'react-router-dom'
import { ExternalLink, Cpu, Shield, AlertTriangle, FileText } from 'lucide-react'
import { Header } from '@/components/layout/header'
import { Footer } from '@/components/layout/footer'
import { Badge, StatusBadge, VisibilityBadge } from '@/components/ui/badge'
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
      <main className="flex-1 container py-8 max-w-3xl space-y-6">
        <Breadcrumbs
          segments={[
            { label: 'Home', href: '/' },
            { label: 'Agents', href: '/agents' },
            { label: data.namespace, href: `/agents?namespace=${data.namespace}` },
            { label: data.slug },
          ]}
        />

        {/* Status message banner */}
        {statusMessage && (
          <div className="flex items-center gap-2 rounded-md border border-yellow-500/30 bg-yellow-500/10 px-4 py-3 text-sm text-yellow-800 dark:text-yellow-200">
            <AlertTriangle className="h-4 w-4 shrink-0" />
            {statusMessage}
          </div>
        )}

        {/* Title row */}
        <div className="space-y-2">
          <div className="flex items-start gap-3 flex-wrap">
            {iconUrl && (
              <img src={iconUrl} alt="" className="h-10 w-10 rounded-lg shrink-0 object-cover" />
            )}
            <h1 className="text-2xl sm:text-3xl font-bold flex-1 min-w-0 break-words">{data.name}</h1>
            <div className="flex gap-2 flex-wrap">
              {lv && (
                <Badge variant="outline" className="font-mono">v{lv.version}</Badge>
              )}
              <StatusBadge status={data.status} />
              <VisibilityBadge visibility={data.visibility} />
            </div>
          </div>
          <div className="flex items-center gap-2">
            <p className="text-sm text-muted-foreground font-mono">
              <Link to={`/agents?namespace=${data.namespace}`} className="hover:text-foreground transition-colors">
                {data.namespace}
              </Link>
              /{data.slug}
            </p>
            <CopyButton value={`${data.namespace}/${data.slug}`} label="Copy identifier" />
          </div>
        </div>

        {data.description && <p className="text-muted-foreground">{data.description}</p>}

        {/* External links */}
        <div className="flex gap-3 flex-wrap">
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
        </div>

        <Separator />

        {/* Tabbed content */}
        <Tabs defaultValue={defaultTab} onValueChange={handleTabChange}>
          <TabsList>
            <TabsTrigger value="overview">Overview</TabsTrigger>
            <TabsTrigger value="skills">
              Skills{lv?.skills && lv.skills.length > 0 ? ` (${lv.skills.length})` : ''}
            </TabsTrigger>
            <TabsTrigger value="json">JSON</TabsTrigger>
          </TabsList>

          {/* ── Overview Tab ── */}
          <TabsContent value="overview" className="space-y-6">
            {/* Metadata grid */}
            <dl className="grid grid-cols-1 sm:grid-cols-2 gap-4 text-sm">
              {lv && (
                <>
                  {lv.endpoint_url && (
                    <>
                      <dt className="text-muted-foreground flex items-center gap-1">
                        Endpoint
                        <TooltipInfo content={getFieldExplanation('endpoint_url') ?? ''} />
                      </dt>
                      <dd className="flex items-center gap-2 min-w-0">
                        <a
                          href={lv.endpoint_url}
                          target="_blank"
                          rel="noopener noreferrer"
                          className="font-mono text-xs hover:underline truncate"
                        >
                          {lv.endpoint_url}
                        </a>
                        <CopyButton value={lv.endpoint_url} label="Copy endpoint URL" />
                      </dd>
                    </>
                  )}
                  {lv.protocol_version && (
                    <>
                      <dt className="text-muted-foreground flex items-center gap-1">
                        A2A protocol
                        <TooltipInfo content={getFieldExplanation('a2a_protocol_version') ?? ''} />
                      </dt>
                      <dd className="font-mono">{lv.protocol_version}</dd>
                    </>
                  )}
                  {lv.published_at && (
                    <>
                      <dt className="text-muted-foreground">Published</dt>
                      <dd>{formatDate(lv.published_at)}</dd>
                    </>
                  )}
                  {lv.default_input_modes && lv.default_input_modes.length > 0 && (
                    <>
                      <dt className="text-muted-foreground">Input modes</dt>
                      <dd className="flex flex-wrap gap-1">
                        {lv.default_input_modes.map((m) => {
                          const info = getModeInfo(m)
                          return (
                            <span key={m} className="flex items-center gap-1">
                              <Badge variant="secondary" className="text-xs">{getModeLabel(m)}</Badge>
                              {info && <TooltipInfo content={info.description} />}
                            </span>
                          )
                        })}
                      </dd>
                    </>
                  )}
                  {lv.default_output_modes && lv.default_output_modes.length > 0 && (
                    <>
                      <dt className="text-muted-foreground">Output modes</dt>
                      <dd className="flex flex-wrap gap-1">
                        {lv.default_output_modes.map((m) => {
                          const info = getModeInfo(m)
                          return (
                            <span key={m} className="flex items-center gap-1">
                              <Badge variant="secondary" className="text-xs">{getModeLabel(m)}</Badge>
                              {info && <TooltipInfo content={info.description} />}
                            </span>
                          )
                        })}
                      </dd>
                    </>
                  )}
                  {lv.authentication && lv.authentication.length > 0 && (
                    <>
                      <dt className="text-muted-foreground flex items-center gap-1">
                        <Shield className="h-3.5 w-3.5" /> Auth schemes
                      </dt>
                      <dd className="flex flex-wrap gap-1">
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
                      </dd>
                    </>
                  )}
                </>
              )}
              {/* Provider info */}
              {provider && typeof provider.organization === 'string' && (
                <>
                  <dt className="text-muted-foreground">Provider</dt>
                  <dd>{provider.organization}</dd>
                </>
              )}
              <dt className="text-muted-foreground">Created</dt>
              <dd>{formatDate(data.created_at)}</dd>
              <dt className="text-muted-foreground">Updated</dt>
              <dd>{formatDate(data.updated_at)}</dd>
            </dl>
          </TabsContent>

          {/* ── Skills Tab ── */}
          <TabsContent value="skills" className="space-y-4">
            {lv?.skills && lv.skills.length > 0 ? (
              <div className="grid gap-3 sm:grid-cols-2">
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

          {/* ── JSON Tab ── */}
          <TabsContent value="json">
            <RawJsonViewer data={data} title="Raw API response" />
          </TabsContent>
        </Tabs>
      </main>
      <Footer />
    </div>
  )
}
