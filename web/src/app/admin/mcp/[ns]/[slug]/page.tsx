import type { Metadata } from "next"
import { notFound, redirect } from "next/navigation"
import Link from "next/link"
import { ArrowLeft, Package } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Badge, StatusBadge, VisibilityBadge } from "@/components/ui/badge"
import { DeprecateButton } from "@/components/admin/deprecate-button"
import { Separator } from "@/components/ui/separator"
import { RawJsonViewer } from "@/components/ui/raw-json-viewer"
import { InstallCommand } from "@/components/ui/install-command"
import { getApiClient } from "@/lib/api-client"
import { formatDate, getInstallCommand, ecosystemLabel, isRemoteTransport } from "@/lib/utils"

interface Props {
  params: Promise<{ ns: string; slug: string }>
}

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { ns, slug } = await params
  const api = await getApiClient()
  const { data } = await api.GET("/api/v1/mcp/servers/{namespace}/{slug}", {
    params: { path: { namespace: ns, slug } },
  })
  return { title: data ? `${data.name} — MCP Server` : `${ns}/${slug}` }
}

export default async function AdminMCPServerPage({ params }: Props) {
  const { ns, slug } = await params
  const api = await getApiClient()
  const { data, error } = await api.GET("/api/v1/mcp/servers/{namespace}/{slug}", {
    params: { path: { namespace: ns, slug } },
  })

  if (error || !data) notFound()

  const lv = data.latest_version

  async function setPublic(formData: FormData) {
    "use server"
    const client = await getApiClient()
    await client.POST("/api/v1/mcp/servers/{namespace}/{slug}/visibility", {
      params: { path: { namespace: ns, slug } },
      body: { visibility: formData.get("visibility") as "public" | "private" },
    })
    redirect(`/admin/mcp/${ns}/${slug}`)
  }

  async function deprecate() {
    "use server"
    const client = await getApiClient()
    await client.POST("/api/v1/mcp/servers/{namespace}/{slug}/deprecate", {
      params: { path: { namespace: ns, slug } },
    })
    redirect(`/admin/mcp/${ns}/${slug}`)
  }

  return (
    <div className="space-y-6 max-w-3xl mx-auto">
      <nav aria-label="Breadcrumb" className="flex items-center gap-1.5 text-sm text-muted-foreground">
        <Link href="/admin/mcp" className="flex items-center gap-1 hover:text-foreground transition-colors">
          <ArrowLeft className="h-3.5 w-3.5" aria-hidden="true" />
          MCP Servers
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
            <dt className="text-muted-foreground">Runtime</dt>
            <dd><Badge variant="secondary">{lv.runtime}</Badge></dd>
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

      {/* Packages */}
      {lv?.packages && lv.packages.length > 0 && (
        <div className="space-y-3">
          <h2 className="text-lg font-semibold flex items-center gap-2">
            <Package className="h-4 w-4" aria-hidden="true" /> Packages
          </h2>
          <div className="space-y-4">
            {lv.packages.map((pkg, i) => {
              const remote = isRemoteTransport(pkg.transport.type)
              return (
                <div key={i} className="space-y-1.5">
                  <div className="flex items-center gap-2 flex-wrap">
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
                  {remote && pkg.transport.url ? (
                    <div className="space-y-1">
                      <p className="text-xs text-muted-foreground">Endpoint URL</p>
                      <InstallCommand command={pkg.transport.url} />
                    </div>
                  ) : (
                    <div className="space-y-1">
                      <p className="text-xs text-muted-foreground">Run command</p>
                      <InstallCommand command={getInstallCommand(pkg)} />
                    </div>
                  )}
                </div>
              )
            })}
          </div>
        </div>
      )}

      <Separator />

      <div className="space-y-3">
        <h2 className="text-lg font-semibold">Actions</h2>
        <div className="flex flex-wrap gap-2">
          {/* Visibility toggle */}
          <form action={setPublic}>
            <input
              type="hidden"
              name="visibility"
              value={data.visibility === "public" ? "private" : "public"}
            />
            <Button variant="outline" size="sm" type="submit">
              Make {data.visibility === "public" ? "private" : "public"}
            </Button>
          </form>

          {/* Deprecate — only valid from published state */}
          {data.status === "published" && (
            <DeprecateButton action={deprecate} entityName={data.name} />
          )}
        </div>
      </div>

      <Separator />

      {/* Raw JSON */}
      <RawJsonViewer data={data} title="Raw API response" />
    </div>
  )
}
