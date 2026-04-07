import type { Metadata } from "next"
import { Search } from "lucide-react"
import { Header } from "@/components/layout/header"
import { Footer } from "@/components/layout/footer"
import { ServerCard } from "@/components/mcp/server-card"
import { Input } from "@/components/ui/input"
import { Button } from "@/components/ui/button"
import { getPublicClient } from "@/lib/api-client"

export const metadata: Metadata = { title: "MCP Servers" }

export default async function MCPPage({
  searchParams,
}: {
  searchParams: Promise<{ q?: string; cursor?: string }>
}) {
  const { q, cursor } = await searchParams
  const api = getPublicClient()
  const result = await api
    .GET("/api/v1/mcp/servers", { params: { query: { q, cursor, limit: 20 } } })
    .catch(() => ({ data: undefined, error: undefined }))

  const { data } = result
  const servers = data?.items ?? []

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

        {/* Search — progressive enhancement, no JS required */}
        <form className="flex gap-2 max-w-sm">
          <div className="relative flex-1">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground pointer-events-none" />
            <Input
              name="q"
              defaultValue={q}
              placeholder="Search servers…"
              className="pl-9"
            />
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

        {servers.length === 0 ? (
          <p className="text-muted-foreground py-12 text-center">
            {q ? `No results for "${q}"` : "No public MCP servers yet."}
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
              <a href={`/mcp?cursor=${data.next_cursor}${q ? `&q=${encodeURIComponent(q)}` : ""}`}>
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
