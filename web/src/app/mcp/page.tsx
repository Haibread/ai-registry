import type { Metadata } from "next"
import { Suspense } from "react"
import { Header } from "@/components/layout/header"
import { Footer } from "@/components/layout/footer"
import { ServerCard } from "@/components/mcp/server-card"
import { FilterBar } from "@/components/ui/filter-bar"
import { Button } from "@/components/ui/button"
import { getPublicClient } from "@/lib/api-client"

export const metadata: Metadata = { title: "MCP Servers" }

export default async function MCPPage({
  searchParams,
}: {
  searchParams: Promise<{
    q?: string
    cursor?: string
    namespace?: string
    status?: string
  }>
}) {
  const { q, cursor, namespace, status } = await searchParams
  const api = getPublicClient()
  const result = await api
    .GET("/api/v1/mcp/servers", {
      params: {
        query: {
          q,
          cursor,
          limit: 20,
          namespace,
          status: status as "draft" | "published" | "deprecated" | undefined,
        },
      },
    })
    .catch(() => ({ data: undefined }))

  const { data } = result
  const servers = data?.items ?? []

  // Build next-page URL preserving all active filters.
  const nextParams = new URLSearchParams()
  if (q) nextParams.set("q", q)
  if (namespace) nextParams.set("namespace", namespace)
  if (status) nextParams.set("status", status)
  if (data?.next_cursor) nextParams.set("cursor", data.next_cursor)

  return (
    <div className="flex min-h-screen flex-col">
      <Header />
      <main className="flex-1 container py-8 space-y-6">
        <div>
          <h1 className="text-2xl font-bold">MCP Servers</h1>
          <p className="text-muted-foreground mt-1">
            Browse Model Context Protocol servers in the registry.
          </p>
        </div>

        <Suspense>
          <FilterBar
            q={q}
            namespace={namespace}
            status={status}
            statusOptions={["published", "deprecated"]}
            searchPlaceholder="Search servers…"
          />
        </Suspense>

        {servers.length === 0 ? (
          <p className="text-muted-foreground py-12 text-center">
            {q || namespace || status
              ? "No servers match your filters."
              : "No public MCP servers yet."}
          </p>
        ) : (
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
            {servers.map((s) => (
              <ServerCard key={s.id} server={s} />
            ))}
          </div>
        )}

        {data?.next_cursor && (
          <div className="flex justify-center">
            <Button variant="outline" asChild>
              <a href={`/mcp?${nextParams.toString()}`}>Load more</a>
            </Button>
          </div>
        )}
      </main>
      <Footer />
    </div>
  )
}
