import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Link, useParams, useNavigate } from 'react-router-dom'
import { ArrowLeft, Cpu, Shield, ExternalLink } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Badge, StatusBadge, VisibilityBadge } from '@/components/ui/badge'
import { DeprecateButton } from '@/components/admin/deprecate-button'
import { Separator } from '@/components/ui/separator'
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '@/components/ui/card'
import { RawJsonViewer } from '@/components/ui/raw-json-viewer'
import { getAuthClient } from '@/lib/api-client'
import { formatDate } from '@/lib/utils'
import { useAuth } from '@/auth/AuthContext'
import type { components } from '@/lib/schema'

type AgentSkill = components['schemas']['AgentSkill']

export default function AdminAgentDetail() {
  const { ns, slug } = useParams<{ ns: string; slug: string }>()
  const { accessToken } = useAuth()
  const navigate = useNavigate()
  const queryClient = useQueryClient()

  const api = getAuthClient(accessToken ?? '')
  const { data, isLoading, isError } = useQuery({
    queryKey: ['admin-agent-detail', ns, slug],
    queryFn: () => api.GET('/api/v1/agents/{namespace}/{slug}', {
      params: { path: { namespace: ns!, slug: slug! } },
    }).then(r => r.data),
    enabled: !!ns && !!slug && !!accessToken,
  })

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ['admin-agent-detail', ns, slug] })
    queryClient.invalidateQueries({ queryKey: ['admin-agents'] })
  }

  const visibilityMutation = useMutation({
    mutationFn: async (newVisibility: 'public' | 'private') => {
      const client = getAuthClient(accessToken!)
      await client.POST('/api/v1/agents/{namespace}/{slug}/visibility', {
        params: { path: { namespace: ns!, slug: slug! } },
        body: { visibility: newVisibility },
      })
    },
    onSuccess: invalidate,
  })

  const deprecateMutation = useMutation({
    mutationFn: async () => {
      const client = getAuthClient(accessToken!)
      await client.POST('/api/v1/agents/{namespace}/{slug}/deprecate', {
        params: { path: { namespace: ns!, slug: slug! } },
      })
    },
    onSuccess: invalidate,
  })

  if (isLoading) return <p className="text-muted-foreground">Loading…</p>
  if (isError || !data) return (
    <div className="space-y-4">
      <p className="text-destructive">Not found.</p>
      <Button variant="outline" size="sm" onClick={() => navigate('/admin/agents')}>Back to Agents</Button>
    </div>
  )

  const lv = data.latest_version

  return (
    <div className="space-y-6 max-w-3xl mx-auto">
      <nav aria-label="Breadcrumb" className="flex items-center gap-1.5 text-sm text-muted-foreground">
        <Link to="/admin/agents" className="flex items-center gap-1 hover:text-foreground transition-colors">
          <ArrowLeft className="h-3.5 w-3.5" aria-hidden="true" />
          Agents
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
            {lv.endpoint_url && (
              <>
                <dt className="text-muted-foreground">Endpoint</dt>
                <dd>
                  <a
                    href={lv.endpoint_url}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="font-mono text-xs hover:underline break-all"
                  >
                    {lv.endpoint_url}
                  </a>
                </dd>
              </>
            )}
            {lv.protocol_version && (
              <>
                <dt className="text-muted-foreground">A2A protocol</dt>
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
                  {lv.default_input_modes.map((m) => (
                    <Badge key={m} variant="secondary" className="text-xs">{m}</Badge>
                  ))}
                </dd>
              </>
            )}
            {lv.default_output_modes && lv.default_output_modes.length > 0 && (
              <>
                <dt className="text-muted-foreground">Output modes</dt>
                <dd className="flex flex-wrap gap-1">
                  {lv.default_output_modes.map((m) => (
                    <Badge key={m} variant="secondary" className="text-xs">{m}</Badge>
                  ))}
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
                      <Badge key={i} variant="outline" className="text-xs">{label}</Badge>
                    )
                  })}
                </dd>
              </>
            )}
          </>
        )}
        <dt className="text-muted-foreground">Created</dt>
        <dd>{formatDate(data.created_at)}</dd>
        <dt className="text-muted-foreground">Updated</dt>
        <dd>{formatDate(data.updated_at)}</dd>
      </dl>

      {/* Skills grid */}
      {lv?.skills && lv.skills.length > 0 && (
        <div className="space-y-3">
          <h2 className="text-lg font-semibold flex items-center gap-2">
            <Cpu className="h-4 w-4" aria-hidden="true" /> Skills
          </h2>
          <div className="grid gap-3 sm:grid-cols-2">
            {lv.skills.map((skill: AgentSkill) => (
              <Card key={skill.id} className="bg-muted/30">
                <CardHeader className="pb-2 pt-4 px-4">
                  <CardTitle className="text-sm">{skill.name}</CardTitle>
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
                            <li key={i} className="truncate">• {ex}</li>
                          ))}
                        </ul>
                      </div>
                    )}
                  </CardContent>
                )}
              </Card>
            ))}
          </div>
        </div>
      )}

      <Separator />

      <div className="space-y-3">
        <h2 className="text-lg font-semibold">Actions</h2>
        <div className="flex flex-wrap gap-2">
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
        </div>
        {(visibilityMutation.isError || deprecateMutation.isError) && (
          <p className="text-sm text-destructive">Action failed. Please try again.</p>
        )}
      </div>

      <Separator />

      <div className="space-y-2">
        <h2 className="text-lg font-semibold">A2A Agent Card</h2>
        <p className="text-sm text-muted-foreground">
          Published at the well-known path for A2A discovery.
        </p>
        <Button variant="outline" size="sm" asChild>
          <a
            href={`/agents/${ns}/${slug}/.well-known/agent-card.json`}
            target="_blank"
            rel="noopener noreferrer"
            className="flex items-center gap-1.5"
          >
            <ExternalLink className="h-4 w-4" /> View agent card
          </a>
        </Button>
      </div>

      <Separator />

      <RawJsonViewer data={data} title="Raw API response" />
    </div>
  )
}
