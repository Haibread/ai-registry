import type { Metadata } from "next"
import { notFound, redirect } from "next/navigation"
import Link from "next/link"
import { ArrowLeft, Package } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Badge, statusVariant, visibilityVariant } from "@/components/ui/badge"
import { Separator } from "@/components/ui/separator"
import { RawJsonViewer } from "@/components/ui/raw-json-viewer"
import { InstallCommand } from "@/components/ui/install-command"
import { getApiClient } from "@/lib/api-client"
import { formatDate, getInstallCommand, ecosystemLabel } from "@/lib/utils"

interface Props {
  params: Promise<{ ns: string; slug: string }>
}

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { ns, slug } = await params
  return { title: `${ns}/${slug} — MCP` }
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
    <div className="space-y-6 max-w-3xl">
      <div className="flex items-center gap-3 flex-wrap">
        <Button variant="ghost" size="sm" asChild>
          <Link href="/admin/mcp" className="flex items-center gap-1">
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
          <h2 className="font-semibold flex items-center gap-2">
            <Package className="h-4 w-4" /> Packages
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

      <Separator />

      <div className="space-y-3">
        <h2 className="font-semibold">Actions</h2>
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
            <form action={deprecate}>
              <Button variant="destructive" size="sm" type="submit">
                Deprecate
              </Button>
            </form>
          )}
        </div>
      </div>

      <Separator />

      {/* Raw JSON */}
      <RawJsonViewer data={data} title="Raw API response" />
    </div>
  )
}
