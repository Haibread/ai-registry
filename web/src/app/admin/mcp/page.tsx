import type { Metadata } from "next"
import { Suspense } from "react"
import Link from "next/link"
import { Plus } from "lucide-react"
import { Button } from "@/components/ui/button"
import { StatusBadge, VisibilityBadge } from "@/components/ui/badge"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { FilterBar } from "@/components/ui/filter-bar"
import { FilterBarSkeleton } from "@/components/ui/filter-bar-skeleton"
import { getApiClient } from "@/lib/api-client"
import { formatDate } from "@/lib/utils"

export const metadata: Metadata = { title: "MCP Servers" }

const PAGE_LIMIT = 50

export default async function AdminMCPPage({
  searchParams,
}: {
  searchParams: Promise<{
    q?: string
    namespace?: string
    status?: string
    visibility?: string
    cursor?: string
  }>
}) {
  const { q, namespace, status, visibility, cursor } = await searchParams
  const api = await getApiClient()
  const { data } = await api.GET("/api/v1/mcp/servers", {
    params: {
      query: {
        limit: PAGE_LIMIT,
        q,
        namespace,
        cursor,
        status: status as "draft" | "published" | "deprecated" | undefined,
        visibility: visibility as "public" | "private" | undefined,
      },
    },
  })
  const servers = data?.items ?? []

  // Build next-page URL preserving all active filters.
  const nextParams = new URLSearchParams()
  if (q) nextParams.set("q", q)
  if (namespace) nextParams.set("namespace", namespace)
  if (status) nextParams.set("status", status)
  if (visibility) nextParams.set("visibility", visibility)
  if (data?.next_cursor) nextParams.set("cursor", data.next_cursor)

  return (
    <div className="space-y-4 max-w-5xl mx-auto">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">MCP Servers</h1>
          <p className="text-muted-foreground mt-1">
            {servers.length}{data?.total_count && data.total_count > servers.length ? ` of ${data.total_count}` : ""} entr{servers.length !== 1 ? "ies" : "y"}
          </p>
        </div>
        <Button asChild>
          <Link href="/admin/mcp/new" className="flex items-center gap-1.5">
            <Plus className="h-4 w-4" /> New Server
          </Link>
        </Button>
      </div>

      <Suspense fallback={<FilterBarSkeleton />}>
        <FilterBar
          q={q}
          namespace={namespace}
          status={status}
          visibility={visibility}
          statusOptions={["draft", "published", "deprecated"]}
          showVisibility
          searchPlaceholder="Search servers…"
        />
      </Suspense>

      {servers.length === 0 ? (
        <p className="text-muted-foreground py-8 text-center">
          {q || namespace || status || visibility
            ? "No servers match your filters."
            : "No MCP servers yet."}
        </p>
      ) : (
        <>
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
                    <StatusBadge status={s.status} />
                  </TableCell>
                  <TableCell>
                    <VisibilityBadge visibility={s.visibility} />
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

          {data?.next_cursor && (
            <div className="flex justify-center pt-2">
              <Button variant="outline" asChild>
                <a href={`/admin/mcp?${nextParams.toString()}`}>Load more</a>
              </Button>
            </div>
          )}
        </>
      )}
    </div>
  )
}
