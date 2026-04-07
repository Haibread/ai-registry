import type { Metadata } from "next"
import Link from "next/link"
import { Plus } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Badge, statusVariant, visibilityVariant } from "@/components/ui/badge"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { getApiClient } from "@/lib/api-client"
import { formatDate } from "@/lib/utils"

export const metadata: Metadata = { title: "MCP Servers" }

export default async function AdminMCPPage() {
  const api = await getApiClient()
  const { data } = await api.GET("/api/v1/mcp/servers", { params: { query: { limit: 100 } } })
  const servers = data?.items ?? []

  return (
    <div className="space-y-4 max-w-5xl">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">MCP Servers</h1>
          <p className="text-muted-foreground mt-1">{servers.length} entries</p>
        </div>
        <Button asChild>
          <Link href="/admin/mcp/new" className="flex items-center gap-1.5">
            <Plus className="h-4 w-4" /> New Server
          </Link>
        </Button>
      </div>

      {servers.length === 0 ? (
        <p className="text-muted-foreground py-8 text-center">No MCP servers yet.</p>
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Name</TableHead>
              <TableHead>Namespace / Slug</TableHead>
              <TableHead>Status</TableHead>
              <TableHead>Visibility</TableHead>
              <TableHead>Updated</TableHead>
              <TableHead />
            </TableRow>
          </TableHeader>
          <TableBody>
            {servers.map((s) => (
              <TableRow key={s.id}>
                <TableCell className="font-medium">{s.name}</TableCell>
                <TableCell className="font-mono text-sm text-muted-foreground">
                  {s.namespace}/{s.slug}
                </TableCell>
                <TableCell>
                  <Badge variant={statusVariant(s.status)}>{s.status}</Badge>
                </TableCell>
                <TableCell>
                  <Badge variant={visibilityVariant(s.visibility)}>{s.visibility}</Badge>
                </TableCell>
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
  )
}
