import type { Metadata } from "next"
import { notFound, redirect } from "next/navigation"
import Link from "next/link"
import { ArrowLeft, Cpu, Shield, ExternalLink } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Badge, statusVariant, visibilityVariant } from "@/components/ui/badge"
import { Separator } from "@/components/ui/separator"
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from "@/components/ui/card"
import { RawJsonViewer } from "@/components/ui/raw-json-viewer"
import { getApiClient } from "@/lib/api-client"
import { formatDate } from "@/lib/utils"
import type { components } from "@/lib/schema"

type AgentSkill = components["schemas"]["AgentSkill"]

interface Props {
  params: Promise<{ ns: string; slug: string }>
}

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { ns, slug } = await params
  return { title: `${ns}/${slug} — Agent` }
}

export default async function AdminAgentPage({ params }: Props) {
  const { ns, slug } = await params
  const api = await getApiClient()
  const { data, error } = await api.GET("/api/v1/agents/{namespace}/{slug}", {
    params: { path: { namespace: ns, slug } },
  })

  if (error || !data) notFound()

  const lv = data.latest_version

  async function setVisibility(formData: FormData) {
    "use server"
    const client = await getApiClient()
    await client.POST("/api/v1/agents/{namespace}/{slug}/visibility", {
      params: { path: { namespace: ns, slug } },
      body: { visibility: formData.get("visibility") as "public" | "private" },
    })
    redirect(`/admin/agents/${ns}/${slug}`)
  }

  async function deprecate() {
    "use server"
    const client = await getApiClient()
    await client.POST("/api/v1/agents/{namespace}/{slug}/deprecate", {
      params: { path: { namespace: ns, slug } },
    })
    redirect(`/admin/agents/${ns}/${slug}`)
  }

  return (
    <div className="space-y-6 max-w-3xl">
      <div className="flex items-center gap-3 flex-wrap">
        <Button variant="ghost" size="sm" asChild>
          <Link href="/admin/agents" className="flex items-center gap-1">
            <ArrowLeft className="h-4 w-4" /> Back
          </Link>
        </Button>
        <h1 className="text-2xl font-bold flex-1">{data.name}</h1>
        <div className="flex gap-2">
          {lv && <Badge variant="outline" className="font-mono">v{lv.version}</Badge>}
          <Badge variant={statusVariant(data.status)}>{data.status}</Badge>
          <Badge variant={visibilityVariant(data.visibility)}>{data.visibility}</Badge>
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
          <h2 className="font-semibold flex items-center gap-2">
            <Cpu className="h-4 w-4" /> Skills
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
        <h2 className="font-semibold">Actions</h2>
        <div className="flex flex-wrap gap-2">
          <form action={setVisibility}>
            <input
              type="hidden"
              name="visibility"
              value={data.visibility === "public" ? "private" : "public"}
            />
            <Button variant="outline" size="sm" type="submit">
              Make {data.visibility === "public" ? "private" : "public"}
            </Button>
          </form>

          {data.status === "published" && (
            <form action={deprecate}>
              <Button variant="destructive" size="sm" type="submit">
                Deprecate
              </Button>
            </form>
          )}
        </div>
      </div>

      <Separator />

      <div className="space-y-2">
        <h2 className="font-semibold">A2A Agent Card</h2>
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

      {/* Raw JSON */}
      <RawJsonViewer data={data} title="Raw API response" />
    </div>
  )
}
