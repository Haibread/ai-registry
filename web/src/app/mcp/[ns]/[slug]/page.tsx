import type { Metadata } from "next"
import { notFound } from "next/navigation"
import Link from "next/link"
import { ExternalLink, GitFork, ArrowLeft, Package } from "lucide-react"
import { Header } from "@/components/layout/header"
import { Footer } from "@/components/layout/footer"
import { Badge, statusVariant, visibilityVariant } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Separator } from "@/components/ui/separator"
import { RawJsonViewer } from "@/components/ui/raw-json-viewer"
import { InstallCommand } from "@/components/ui/install-command"
import { getPublicClient } from "@/lib/api-client"
import { formatDate, getInstallCommand, ecosystemLabel } from "@/lib/utils"

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

  const lv = data.latest_version

  return (
    <div className="flex min-h-screen flex-col">
      <Header />
      <main className="flex-1 container py-8 max-w-3xl space-y-6">
        <Button variant="ghost" size="sm" asChild>
          <Link href="/mcp" className="flex items-center gap-1">
            <ArrowLeft className="h-4 w-4" /> Back
          </Link>
        </Button>

        {/* Title row */}
        <div className="space-y-2">
          <div className="flex items-start gap-3 flex-wrap">
            <h1 className="text-2xl font-bold flex-1">{data.name}</h1>
            <div className="flex gap-2 flex-wrap">
              {lv && (
                <Badge variant="outline" className="font-mono">v{lv.version}</Badge>
              )}
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

        {/* Metadata grid */}
        <dl className="grid grid-cols-2 gap-4 text-sm">
          {lv && (
            <>
              <dt className="text-muted-foreground">Runtime</dt>
              <dd>
                <Badge variant="secondary">{lv.runtime}</Badge>
              </dd>
              <dt className="text-muted-foreground">Protocol version</dt>
              <dd className="font-mono">{lv.protocol_version}</dd>
              {lv.published_at && (
                <>
                  <dt className="text-muted-foreground">Published</dt>
                  <dd>{formatDate(lv.published_at)}</dd>
                </>
              )}
            </>
          )}
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

        {/* Packages / Install commands */}
        {lv?.packages && lv.packages.length > 0 && (
          <div className="space-y-3">
            <h2 className="font-semibold flex items-center gap-2">
              <Package className="h-4 w-4" /> Installation
            </h2>
            <div className="space-y-3">
              {lv.packages.map((pkg, i) => (
                <div key={i} className="space-y-1.5">
                  <div className="flex items-center gap-2">
                    <Badge variant="secondary" className="text-xs">
                      {ecosystemLabel(pkg.registryType)}
                    </Badge>
                    <span className="text-xs text-muted-foreground font-mono">
                      {pkg.identifier}@{pkg.version}
                    </span>
                    <Badge variant="outline" className="text-xs">
                      {pkg.transport.type}
                    </Badge>
                  </div>
                  <InstallCommand command={getInstallCommand(pkg)} />
                </div>
              ))}
            </div>
          </div>
        )}

        {/* External links */}
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

        <Separator />

        {/* Raw JSON viewer */}
        <RawJsonViewer data={data} title="Raw API response" />
      </main>
      <Footer />
    </div>
  )
}
