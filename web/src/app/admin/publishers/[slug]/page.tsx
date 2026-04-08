import type { Metadata } from "next"
import { notFound } from "next/navigation"
import Link from "next/link"
import { ArrowLeft, CheckCircle2, Circle, Server, Bot } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Separator } from "@/components/ui/separator"
import { Badge } from "@/components/ui/badge"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { StatusBadge } from "@/components/ui/badge"
import { getApiClient } from "@/lib/api-client"
import { formatDate } from "@/lib/utils"

interface Props {
  params: Promise<{ slug: string }>
}

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { slug } = await params
  const api = await getApiClient()
  const { data } = await api.GET("/api/v1/publishers/{slug}", {
    params: { path: { slug } },
  })
  return { title: data ? `${data.name} — Publisher` : slug }
}

export default async function AdminPublisherPage({ params }: Props) {
  const { slug } = await params
  const api = await getApiClient()

  const [publisherRes, mcpRes, agentsRes] = await Promise.all([
    api.GET("/api/v1/publishers/{slug}", { params: { path: { slug } } }),
    api.GET("/api/v1/mcp/servers", { params: { query: { namespace: slug, limit: 50 } } }),
    api.GET("/api/v1/agents", { params: { query: { namespace: slug, limit: 50 } } }),
  ])

  if (publisherRes.error || !publisherRes.data) notFound()

  const publisher = publisherRes.data
  const mcpServers = mcpRes.data?.items ?? []
  const agents = agentsRes.data?.items ?? []

  return (
    <div className="space-y-6 max-w-4xl">
      <nav aria-label="Breadcrumb" className="flex items-center gap-1.5 text-sm text-muted-foreground">
        <Link href="/admin/publishers" className="flex items-center gap-1 hover:text-foreground transition-colors">
          <ArrowLeft className="h-3.5 w-3.5" aria-hidden="true" />
          Publishers
        </Link>
        <span aria-hidden="true">/</span>
        <span className="font-mono text-foreground">{publisher.slug}</span>
      </nav>

      {/* Header */}
      <div className="flex items-start gap-3 flex-wrap">
        <div className="flex-1">
          <h1 className="text-2xl font-bold">{publisher.name}</h1>
          <p className="text-sm text-muted-foreground font-mono mt-0.5">{publisher.slug}</p>
        </div>
        {publisher.verified ? (
          <Badge variant="success" className="gap-1">
            <CheckCircle2 className="h-3 w-3" aria-hidden="true" /> Verified
          </Badge>
        ) : (
          <Badge variant="muted" className="gap-1">
            <Circle className="h-3 w-3" aria-hidden="true" /> Unverified
          </Badge>
        )}
      </div>

      {/* Metadata */}
      <dl className="grid grid-cols-2 gap-x-4 gap-y-2 text-sm max-w-sm">
        {publisher.contact && (
          <>
            <dt className="text-muted-foreground">Contact</dt>
            <dd>{publisher.contact}</dd>
          </>
        )}
        <dt className="text-muted-foreground">Created</dt>
        <dd>{formatDate(publisher.created_at)}</dd>
        <dt className="text-muted-foreground">Updated</dt>
        <dd>{formatDate(publisher.updated_at)}</dd>
      </dl>

      <Separator />

      {/* MCP Servers */}
      <div className="space-y-3">
        <div className="flex items-center justify-between">
          <h2 className="text-lg font-semibold flex items-center gap-2">
            <Server className="h-4 w-4" aria-hidden="true" />
            MCP Servers
            <span className="text-sm font-normal text-muted-foreground">({mcpServers.length})</span>
          </h2>
          <Button size="sm" asChild>
            <Link href={`/admin/mcp/new`}>New Server</Link>
          </Button>
        </div>

        {mcpServers.length === 0 ? (
          <p className="text-sm text-muted-foreground py-4">No MCP servers under this namespace.</p>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Slug</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Updated</TableHead>
                <TableHead />
              </TableRow>
            </TableHeader>
            <TableBody>
              {mcpServers.map((s) => (
                <TableRow key={s.id}>
                  <TableCell className="font-medium">{s.name}</TableCell>
                  <TableCell className="font-mono text-sm text-muted-foreground">{s.slug}</TableCell>
                  <TableCell><StatusBadge status={s.status} /></TableCell>
                  <TableCell className="text-muted-foreground">{formatDate(s.updated_at)}</TableCell>
                  <TableCell>
                    <Button variant="ghost" size="sm" asChild>
                      <Link href={`/admin/mcp/${s.namespace}/${s.slug}`}>Manage</Link>
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </div>

      <Separator />

      {/* Agents */}
      <div className="space-y-3">
        <div className="flex items-center justify-between">
          <h2 className="text-lg font-semibold flex items-center gap-2">
            <Bot className="h-4 w-4" aria-hidden="true" />
            Agents
            <span className="text-sm font-normal text-muted-foreground">({agents.length})</span>
          </h2>
          <Button size="sm" asChild>
            <Link href={`/admin/agents/new`}>New Agent</Link>
          </Button>
        </div>

        {agents.length === 0 ? (
          <p className="text-sm text-muted-foreground py-4">No agents under this namespace.</p>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Slug</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Updated</TableHead>
                <TableHead />
              </TableRow>
            </TableHeader>
            <TableBody>
              {agents.map((a) => (
                <TableRow key={a.id}>
                  <TableCell className="font-medium">{a.name}</TableCell>
                  <TableCell className="font-mono text-sm text-muted-foreground">{a.slug}</TableCell>
                  <TableCell><StatusBadge status={a.status} /></TableCell>
                  <TableCell className="text-muted-foreground">{formatDate(a.updated_at)}</TableCell>
                  <TableCell>
                    <Button variant="ghost" size="sm" asChild>
                      <Link href={`/admin/agents/${a.namespace}/${a.slug}`}>Manage</Link>
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </div>
    </div>
  )
}
