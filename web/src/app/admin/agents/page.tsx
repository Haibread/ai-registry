import type { Metadata } from "next"
import Link from "next/link"
import { Plus } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Badge, statusVariant, visibilityVariant } from "@/components/ui/badge"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { getApiClient } from "@/lib/api-client"
import { formatDate } from "@/lib/utils"

export const metadata: Metadata = { title: "Agents" }

export default async function AdminAgentsPage() {
  const api = await getApiClient()
  const { data } = await api.GET("/api/v1/agents", { params: { query: { limit: 100 } } })
  const agents = data?.items ?? []

  return (
    <div className="space-y-4 max-w-5xl">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">Agents</h1>
          <p className="text-muted-foreground mt-1">{agents.length} entries</p>
        </div>
        <Button asChild>
          <Link href="/admin/agents/new" className="flex items-center gap-1.5">
            <Plus className="h-4 w-4" /> New Agent
          </Link>
        </Button>
      </div>

      {agents.length === 0 ? (
        <p className="text-muted-foreground py-8 text-center">No agents yet.</p>
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
            {agents.map((a) => (
              <TableRow key={a.id}>
                <TableCell className="font-medium">{a.name}</TableCell>
                <TableCell className="font-mono text-sm text-muted-foreground">
                  {a.namespace}/{a.slug}
                </TableCell>
                <TableCell>
                  <Badge variant={statusVariant(a.status)}>{a.status}</Badge>
                </TableCell>
                <TableCell>
                  <Badge variant={visibilityVariant(a.visibility)}>{a.visibility}</Badge>
                </TableCell>
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
  )
}
