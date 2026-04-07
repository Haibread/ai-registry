import type { Metadata } from "next"
import { notFound, redirect } from "next/navigation"
import Link from "next/link"
import { ArrowLeft } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Badge, statusVariant, visibilityVariant } from "@/components/ui/badge"
import { Separator } from "@/components/ui/separator"
import { getApiClient } from "@/lib/api-client"
import { formatDate } from "@/lib/utils"

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

  async function setVisibility(formData: FormData) {
    "use server"
    await fetch(
      `${process.env.API_URL}/api/v1/agents/${ns}/${slug}/visibility`,
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ visibility: formData.get("visibility") }),
      }
    )
    redirect(`/admin/agents/${ns}/${slug}`)
  }

  async function deprecate() {
    "use server"
    await fetch(
      `${process.env.API_URL}/api/v1/agents/${ns}/${slug}/deprecate`,
      { method: "POST" }
    )
    redirect(`/admin/agents/${ns}/${slug}`)
  }

  return (
    <div className="space-y-6 max-w-3xl">
      <div className="flex items-center gap-3">
        <Button variant="ghost" size="sm" asChild>
          <Link href="/admin/agents" className="flex items-center gap-1">
            <ArrowLeft className="h-4 w-4" /> Back
          </Link>
        </Button>
        <h1 className="text-2xl font-bold flex-1">{data.name}</h1>
        <Badge variant={statusVariant(data.status)}>{data.status}</Badge>
        <Badge variant={visibilityVariant(data.visibility)}>{data.visibility}</Badge>
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
        <dt className="text-muted-foreground">Created</dt>
        <dd>{formatDate(data.created_at)}</dd>
        <dt className="text-muted-foreground">Updated</dt>
        <dd>{formatDate(data.updated_at)}</dd>
      </dl>

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
          >
            View agent card
          </a>
        </Button>
      </div>
    </div>
  )
}
