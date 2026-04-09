import { cache } from "react"
import type { Metadata } from "next"
import { notFound } from "next/navigation"
import Link from "next/link"
import { ArrowLeft, ExternalLink, Cpu, Shield } from "lucide-react"
import { Header } from "@/components/layout/header"
import { Footer } from "@/components/layout/footer"
import { Badge, StatusBadge, VisibilityBadge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Separator } from "@/components/ui/separator"
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from "@/components/ui/card"
import { RawJsonViewer } from "@/components/ui/raw-json-viewer"
import { getPublicClient } from "@/lib/api-client"
import { formatDate } from "@/lib/utils"
import type { components } from "@/lib/schema"

type AgentSkill = components["schemas"]["AgentSkill"]

interface Props {
  params: Promise<{ ns: string; slug: string }>
}

// cache() deduplicates this fetch within a single request cycle so that
// generateMetadata and the page component share one backend call.
const getAgent = cache(async (ns: string, slug: string) => {
  const api = getPublicClient()
  return api.GET("/api/v1/agents/{namespace}/{slug}", {
    params: { path: { namespace: ns, slug } },
  })
})

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { ns, slug } = await params
  const { data } = await getAgent(ns, slug)
  return { title: data ? `${data.name} — Agent` : `${ns}/${slug}` }
}

export default async function AgentPage({ params }: Props) {
  const { ns, slug } = await params
  const { data, error } = await getAgent(ns, slug)

  if (error || !data) notFound()

  const lv = data.latest_version
  const cardUrl = `/agents/${ns}/${slug}/.well-known/agent-card.json`

  return (
    <div className="flex min-h-screen flex-col">
      <Header />
      <main className="flex-1 container py-8 max-w-3xl space-y-6">
        {/* Breadcrumb */}
        <nav aria-label="Breadcrumb" className="flex items-center gap-1.5 text-sm text-muted-foreground">
          <Link href="/agents" className="flex items-center gap-1 hover:text-foreground transition-colors">
            <ArrowLeft className="h-3.5 w-3.5" aria-hidden="true" />
            Agents
          </Link>
          <span aria-hidden="true">/</span>
          <span className="font-mono text-foreground">{data.namespace}/{data.slug}</span>
        </nav>

        {/* Title row */}
        <div className="space-y-2">
          <div className="flex items-start gap-3 flex-wrap">
            <h1 className="text-3xl font-bold flex-1">{data.name}</h1>
            <div className="flex gap-2 flex-wrap">
              {lv && (
                <Badge variant="outline" className="font-mono">v{lv.version}</Badge>
              )}
              <StatusBadge status={data.status} />
              <VisibilityBadge visibility={data.visibility} />
            </div>
          </div>
          <p className="text-sm text-muted-foreground font-mono">
            {data.namespace}/{data.slug}
          </p>
        </div>

        {data.description && <p className="text-muted-foreground">{data.description}</p>}

        <Separator />

        {/* Metadata grid */}
        <dl className="grid grid-cols-2 gap-4 text-sm">
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
                      const label = s["scheme"] ?? s["type"] ?? `scheme ${i + 1}`
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

        {/* Links */}
        <div className="flex gap-3 flex-wrap">
          <Button variant="outline" size="sm" asChild>
            <a href={cardUrl} target="_blank" rel="noopener noreferrer" className="flex items-center gap-1.5">
              <ExternalLink className="h-4 w-4" /> A2A Agent Card
            </a>
          </Button>
        </div>

        <Separator />

        {/* Raw JSON viewer */}
        <RawJsonViewer data={data} title="Raw API response" />
      </main>
      <Footer />
    </div>
  )
}
