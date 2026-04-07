import type { Metadata } from "next"
import { notFound } from "next/navigation"
import Link from "next/link"
import { ExternalLink, GitFork, ArrowLeft } from "lucide-react"
import { Header } from "@/components/layout/header"
import { Footer } from "@/components/layout/footer"
import { Badge, statusVariant, visibilityVariant } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Separator } from "@/components/ui/separator"
import { getPublicClient } from "@/lib/api-client"
import { formatDate } from "@/lib/utils"

interface Props {
  params: Promise<{ ns: string; slug: string }>
}

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { ns, slug } = await params
  return { title: `${ns}/${slug}` }
}

export default async function MCPServerPage({ params }: Props) {
  const { ns, slug } = await params
  const api = getPublicClient()
  const { data, error } = await api.GET("/api/v1/mcp/servers/{namespace}/{slug}", {
    params: { path: { namespace: ns, slug } },
  })

  if (error || !data) notFound()

  return (
    <div className="flex min-h-screen flex-col">
      <Header />
      <main className="flex-1 container py-8 max-w-3xl space-y-6">
        <Button variant="ghost" size="sm" asChild>
          <Link href="/mcp" className="flex items-center gap-1">
            <ArrowLeft className="h-4 w-4" /> Back
          </Link>
        </Button>

        <div className="space-y-2">
          <div className="flex items-start gap-3 flex-wrap">
            <h1 className="text-2xl font-bold flex-1">{data.name}</h1>
            <div className="flex gap-2">
              <Badge variant={statusVariant(data.status)}>{data.status}</Badge>
              <Badge variant={visibilityVariant(data.visibility)}>{data.visibility}</Badge>
            </div>
          </div>
          <p className="text-sm text-muted-foreground font-mono">
            {data.namespace}/{data.slug}
          </p>
        </div>

        {data.description && <p className="text-muted-foreground">{data.description}</p>}

        <Separator />

        <dl className="grid grid-cols-2 gap-4 text-sm">
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

        <div className="flex gap-3 flex-wrap">
          {data.repo_url && (
            <Button variant="outline" size="sm" asChild>
              <a href={data.repo_url} target="_blank" rel="noopener noreferrer" className="flex items-center gap-1.5">
                <GitFork className="h-4 w-4" /> Repository
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
      </main>
      <Footer />
    </div>
  )
}
