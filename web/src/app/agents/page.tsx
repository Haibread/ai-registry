import type { Metadata } from "next"
import { Search } from "lucide-react"
import { Header } from "@/components/layout/header"
import { Footer } from "@/components/layout/footer"
import { AgentCard } from "@/components/agents/agent-card"
import { Input } from "@/components/ui/input"
import { Button } from "@/components/ui/button"
import { getPublicClient } from "@/lib/api-client"

export const metadata: Metadata = { title: "Agents" }

export default async function AgentsPage({
  searchParams,
}: {
  searchParams: Promise<{ q?: string; cursor?: string }>
}) {
  const { q, cursor } = await searchParams
  const api = getPublicClient()
  const result = await api
    .GET("/api/v1/agents", { params: { query: { q, cursor, limit: 20 } } })
    .catch(() => ({ data: undefined, error: undefined }))

  const { data } = result
  const agents = data?.items ?? []

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

        <form className="flex gap-2 max-w-sm">
          <div className="relative flex-1">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground pointer-events-none" />
            <Input name="q" defaultValue={q} placeholder="Search agents…" className="pl-9" />
          </div>
          <Button type="submit" variant="secondary">
            Search
          </Button>
          {q && (
            <Button type="submit" variant="ghost" name="q" value="">
              Clear
            </Button>
          )}
        </form>

        {agents.length === 0 ? (
          <p className="text-muted-foreground py-12 text-center">
            {q ? `No results for "${q}"` : "No public agents yet."}
          </p>
        ) : (
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
            {agents.map((a) => (
              <AgentCard key={a.id} agent={a} />
            ))}
          </div>
        )}

        {data?.next_cursor && (
          <div className="flex justify-center">
            <Button variant="outline" asChild>
              <a href={`/agents?cursor=${data.next_cursor}${q ? `&q=${encodeURIComponent(q)}` : ""}`}>
                Load more
              </a>
            </Button>
          </div>
        )}
      </main>
      <Footer />
    </div>
  )
}
