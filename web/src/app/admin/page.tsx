import type { Metadata } from "next"
import Link from "next/link"
import { Server, Bot, Users, ArrowRight, Plus } from "lucide-react"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { StatusBadge } from "@/components/ui/badge"
import { Separator } from "@/components/ui/separator"
import { getApiClient } from "@/lib/api-client"
import { formatDate } from "@/lib/utils"

export const metadata: Metadata = { title: "Admin Dashboard" }

export default async function AdminDashboardPage() {
  const api = await getApiClient()

  const [statsRes, mcpRes, agentsRes] = await Promise.all([
    api.GET("/api/v1/stats"),
    api.GET("/api/v1/mcp/servers", { params: { query: { limit: 5 } } }),
    api.GET("/api/v1/agents", { params: { query: { limit: 5 } } }),
  ])

  const { data, response } = statsRes
  const recentMcp = mcpRes.data?.items ?? []
  const recentAgents = agentsRes.data?.items ?? []

  const statsError =
    !data && response && response.status !== 200
      ? `Failed to load stats (HTTP ${response.status}) — check backend connectivity and Keycloak token claims.`
      : null

  const stats = [
    { label: "MCP Servers", value: data?.mcp_servers ?? "—", icon: Server, href: "/admin/mcp" },
    { label: "Agents",      value: data?.agents      ?? "—", icon: Bot,    href: "/admin/agents" },
    { label: "Publishers",  value: data?.publishers  ?? "—", icon: Users,  href: "/admin/publishers" },
  ]

  const quickActions = [
    { label: "New Publisher",  href: "/admin/publishers/new", icon: Users  },
    { label: "New MCP Server", href: "/admin/mcp/new",        icon: Server },
    { label: "New Agent",      href: "/admin/agents/new",     icon: Bot    },
  ]

  return (
    <div className="space-y-8 max-w-4xl mx-auto">
      <div>
        <h1 className="text-2xl font-bold">Dashboard</h1>
        <p className="text-muted-foreground mt-1">Registry overview and quick actions.</p>
      </div>

      {statsError && (
        <div
          role="alert"
          className="rounded-md border border-destructive/50 bg-destructive/10 px-4 py-3 text-sm text-destructive"
          data-testid="stats-error"
        >
          {statsError}
        </div>
      )}

      {/* Stat cards */}
      <div className="grid gap-4 sm:grid-cols-3">
        {stats.map(({ label, value, icon: Icon, href }) => (
          <Card key={label}>
            <CardHeader className="pb-2">
              <CardTitle className="text-sm font-medium text-muted-foreground flex items-center gap-2">
                <Icon className="h-4 w-4" />
                {label}
              </CardTitle>
            </CardHeader>
            <CardContent className="flex items-end justify-between">
              <p className="text-3xl font-bold">{value}</p>
              <Button variant="ghost" size="sm" asChild>
                <Link href={href} className="flex items-center gap-1 text-xs">
                  Manage <ArrowRight className="h-3 w-3" />
                </Link>
              </Button>
            </CardContent>
          </Card>
        ))}
      </div>

      <Separator />

      {/* Quick actions */}
      <div className="space-y-3">
        <h2 className="text-sm font-semibold text-muted-foreground uppercase tracking-wide">Quick Actions</h2>
        <div className="flex flex-wrap gap-3">
          {quickActions.map(({ label, href, icon: Icon }) => (
            <Button key={href} variant="outline" asChild>
              <Link href={href} className="flex items-center gap-2">
                <Plus className="h-4 w-4" />
                <Icon className="h-4 w-4" />
                {label}
              </Link>
            </Button>
          ))}
        </div>
      </div>

      <Separator />

      {/* Recent entries */}
      <div className="grid gap-6 sm:grid-cols-2">
        {/* Recent MCP servers */}
        <div className="space-y-3">
          <div className="flex items-center justify-between">
            <h2 className="text-sm font-semibold text-muted-foreground uppercase tracking-wide">Recent MCP Servers</h2>
            <Button variant="ghost" size="sm" asChild>
              <Link href="/admin/mcp" className="flex items-center gap-1 text-xs">
                View all <ArrowRight className="h-3 w-3" />
              </Link>
            </Button>
          </div>
          {recentMcp.length === 0 ? (
            <p className="text-sm text-muted-foreground py-4">No MCP servers yet.</p>
          ) : (
            <div className="space-y-1">
              {recentMcp.map((s) => (
                <Link
                  key={s.id}
                  href={`/admin/mcp/${s.namespace}/${s.slug}`}
                  className="flex items-center justify-between rounded-md px-3 py-2 text-sm hover:bg-accent transition-colors"
                >
                  <div className="min-w-0">
                    <p className="font-medium truncate">{s.name}</p>
                    <p className="text-xs text-muted-foreground font-mono truncate">{s.namespace}/{s.slug}</p>
                  </div>
                  <div className="flex items-center gap-2 shrink-0 ml-2">
                    <StatusBadge status={s.status} className="text-[10px]" />
                    <span className="text-xs text-muted-foreground hidden sm:block">{formatDate(s.updated_at)}</span>
                  </div>
                </Link>
              ))}
            </div>
          )}
        </div>

        {/* Recent agents */}
        <div className="space-y-3">
          <div className="flex items-center justify-between">
            <h2 className="text-sm font-semibold text-muted-foreground uppercase tracking-wide">Recent Agents</h2>
            <Button variant="ghost" size="sm" asChild>
              <Link href="/admin/agents" className="flex items-center gap-1 text-xs">
                View all <ArrowRight className="h-3 w-3" />
              </Link>
            </Button>
          </div>
          {recentAgents.length === 0 ? (
            <p className="text-sm text-muted-foreground py-4">No agents yet.</p>
          ) : (
            <div className="space-y-1">
              {recentAgents.map((a) => (
                <Link
                  key={a.id}
                  href={`/admin/agents/${a.namespace}/${a.slug}`}
                  className="flex items-center justify-between rounded-md px-3 py-2 text-sm hover:bg-accent transition-colors"
                >
                  <div className="min-w-0">
                    <p className="font-medium truncate">{a.name}</p>
                    <p className="text-xs text-muted-foreground font-mono truncate">{a.namespace}/{a.slug}</p>
                  </div>
                  <div className="flex items-center gap-2 shrink-0 ml-2">
                    <StatusBadge status={a.status} className="text-[10px]" />
                    <span className="text-xs text-muted-foreground hidden sm:block">{formatDate(a.updated_at)}</span>
                  </div>
                </Link>
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
