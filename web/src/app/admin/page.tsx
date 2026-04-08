import type { Metadata } from "next"
import Link from "next/link"
import { Server, Bot, Users, ArrowRight } from "lucide-react"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { getApiClient } from "@/lib/api-client"

export const metadata: Metadata = { title: "Admin Dashboard" }

export default async function AdminDashboardPage() {
  const api = await getApiClient()
  const { data, response } = await api.GET("/api/v1/stats")

  // Surface auth/connectivity errors so they're visible during development.
  // In production these would go to an error boundary, but a banner is enough.
  const statsError =
    !data && response && response.status !== 200
      ? `Failed to load stats (HTTP ${response.status}) — check backend connectivity and Keycloak token claims.`
      : null

  const stats = [
    {
      label: "MCP Servers",
      value: data?.mcp_servers ?? "—",
      icon: Server,
      href: "/admin/mcp",
    },
    {
      label: "Agents",
      value: data?.agents ?? "—",
      icon: Bot,
      href: "/admin/agents",
    },
    {
      label: "Publishers",
      value: data?.publishers ?? "—",
      icon: Users,
      href: "/admin/publishers",
    },
  ]

  return (
    <div className="space-y-6 max-w-4xl">
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
    </div>
  )
}
