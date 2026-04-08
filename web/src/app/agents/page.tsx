import type { Metadata } from "next"
import { Suspense } from "react"
import { Bot } from "lucide-react"
import { Header } from "@/components/layout/header"
import { Footer } from "@/components/layout/footer"
import { AgentCard } from "@/components/agents/agent-card"
import { FilterBar } from "@/components/ui/filter-bar"
import { FilterBarSkeleton } from "@/components/ui/filter-bar-skeleton"
import { Button } from "@/components/ui/button"
import { getPublicClient } from "@/lib/api-client"

export const metadata: Metadata = { title: "Agents" }

export default async function AgentsPage({
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
    .GET("/api/v1/agents", {
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
  const agents = data?.items ?? []

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
          <h1 className="text-2xl font-bold">AI Agents</h1>
          <p className="text-muted-foreground mt-1">
            Browse A2A-compatible agents in the registry.
          </p>
        </div>

        <Suspense fallback={<FilterBarSkeleton />}>
          <FilterBar
            q={q}
            namespace={namespace}
            status={status}
            statusOptions={["published", "deprecated"]}
            searchPlaceholder="Search agents…"
          />
        </Suspense>

        {agents.length === 0 ? (
          <div className="flex flex-col items-center gap-3 py-16 text-center">
            <Bot className="h-10 w-10 text-muted-foreground/40" aria-hidden="true" />
            <p className="text-muted-foreground font-medium">
              {q || namespace || status
                ? "No agents match your filters."
                : "No public agents yet."}
            </p>
            {(q || namespace || status) && (
              <Button variant="outline" size="sm" asChild>
                <a href="/agents">Clear filters</a>
              </Button>
            )}
          </div>
        ) : (
          <>
            <p className="text-sm text-muted-foreground">
              Showing {agents.length}{data?.total_count && data.total_count > agents.length ? ` of ${data.total_count}` : ""} agent{agents.length !== 1 ? "s" : ""}
            </p>
            <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
              {agents.map((a) => (
                <AgentCard key={a.id} agent={a} />
              ))}
            </div>
          </>
        )}

        {data?.next_cursor && (
          <div className="flex justify-center">
            <Button variant="outline" asChild>
              <a href={`/agents?${nextParams.toString()}`}>Load more</a>
            </Button>
          </div>
        )}
      </main>
      <Footer />
    </div>
  )
}
