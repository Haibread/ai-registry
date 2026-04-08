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

export const metadata: Metadata = { title: "Agents" }

const PAGE_LIMIT = 50

export default async function AdminAgentsPage({
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
  const { data } = await api.GET("/api/v1/agents", {
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
  const agents = data?.items ?? []

  // Build next-page URL preserving all active filters.
  const nextParams = new URLSearchParams()
  if (q) nextParams.set("q", q)
  if (namespace) nextParams.set("namespace", namespace)
  if (status) nextParams.set("status", status)
  if (visibility) nextParams.set("visibility", visibility)
  if (data?.next_cursor) nextParams.set("cursor", data.next_cursor)

  return (
    <div className="space-y-4 max-w-5xl">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">Agents</h1>
          <p className="text-muted-foreground mt-1">
            {agents.length}{data?.total_count && data.total_count > agents.length ? ` of ${data.total_count}` : ""} entr{agents.length !== 1 ? "ies" : "y"}
          </p>
        </div>
        <Button asChild>
          <Link href="/admin/agents/new" className="flex items-center gap-1.5">
            <Plus className="h-4 w-4" /> New Agent
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
          searchPlaceholder="Search agents…"
        />
      </Suspense>

      {agents.length === 0 ? (
        <p className="text-muted-foreground py-8 text-center">
          {q || namespace || status || visibility
            ? "No agents match your filters."
            : "No agents yet."}
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
              {agents.map((a) => (
                <TableRow key={a.id}>
                  <TableCell className="font-medium">{a.name}</TableCell>
                  <TableCell className="font-mono text-sm text-muted-foreground">
                    {a.namespace}/{a.slug}
                  </TableCell>
                  <TableCell>
                    <StatusBadge status={a.status} />
                  </TableCell>
                  <TableCell>
                    <VisibilityBadge visibility={a.visibility} />
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

          {data?.next_cursor && (
            <div className="flex justify-center pt-2">
              <Button variant="outline" asChild>
                <a href={`/admin/agents?${nextParams.toString()}`}>Load more</a>
              </Button>
            </div>
          )}
        </>
      )}
    </div>
  )
}
