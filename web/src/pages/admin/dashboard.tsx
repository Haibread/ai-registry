import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { Server, Bot, Users, ArrowRight, FileText, CheckCircle, AlertTriangle, ClipboardCheck } from 'lucide-react'
import { Card, CardContent, CardHeader } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge, StatusBadge } from '@/components/ui/badge'
import { Separator } from '@/components/ui/separator'
import { useAuthClient } from '@/lib/api-client'
import { formatDate } from '@/lib/utils'
import { usePermissions } from '@/auth/useMe'
import { usePublisher } from '@/auth/PublisherContext'
import { PublisherOverview } from '@/components/admin/publisher-overview'

// AdminDashboard routes the /admin landing page by the selected scope: a
// publisher member sees the scoped PublisherOverview; a Server Admin viewing
// "All publishers" sees the global registry dashboard; a global-only reviewer
// (review grant, no publisher membership) sees their queue; a caller with no
// roles at all sees a short empty state.
export default function AdminDashboard() {
  const { currentSlug, current, isServerAdmin, isLoading } = usePublisher()
  const perms = usePermissions()
  if (isLoading) {
    return <p className="text-muted-foreground">Loading…</p>
  }
  if (currentSlug) {
    return <PublisherOverview slug={currentSlug} option={current} />
  }
  if (isServerAdmin) {
    return <GlobalDashboard />
  }
  // A reviewer with only a global grant has no publisher to scope to, but
  // does have work to do — land them on their queue, not "No publishers
  // yet" (P2.4).
  if (perms.isReviewerAnywhere) {
    return <ReviewerLanding />
  }
  return (
    <div className="mx-auto max-w-md space-y-2 py-16 text-center">
      <h1 className="text-xl font-semibold">No publishers yet</h1>
      <p className="text-sm text-muted-foreground">
        You do not have a role on any publisher. Ask a publisher admin for an Editor or Viewer grant.
      </p>
    </div>
  )
}

function ReviewerLanding() {
  const api = useAuthClient()
  // Same key and shape as the sidebar badge query so the cache is shared.
  const { data } = useQuery({
    queryKey: ['admin-review-queue-count'],
    queryFn: async () => {
      const r = await api.GET('/api/v1/review-queue', {
        params: { query: { limit: 99 } },
      })
      const items = r.data?.items ?? []
      return { count: items.length, hasMore: !!r.data?.next_cursor }
    },
    enabled: true,
  })
  const display = data ? (data.hasMore ? '99+' : String(data.count)) : '…'

  return (
    <div className="mx-auto max-w-md space-y-6 py-16">
      <div className="space-y-2 text-center">
        <h1 className="text-xl font-semibold">Review queue</h1>
        <p className="text-sm text-muted-foreground">
          You have a Reviewer grant. Submissions across all publishers land on
          your queue for approval.
        </p>
      </div>
      <Card>
        <CardContent className="flex items-center justify-between gap-4 pt-6">
          <div className="flex items-center gap-3">
            <ClipboardCheck className="h-8 w-8 text-muted-foreground" aria-hidden="true" />
            <div>
              <p className="text-2xl font-bold tabular-nums">{display}</p>
              <p className="text-sm text-muted-foreground">
                item{data?.count === 1 && !data.hasMore ? '' : 's'} awaiting your review
              </p>
            </div>
          </div>
          <Button asChild>
            <Link to="/admin/review" className="flex items-center gap-1.5">
              Open queue <ArrowRight className="h-4 w-4" aria-hidden="true" />
            </Link>
          </Button>
        </CardContent>
      </Card>
    </div>
  )
}

function GlobalDashboard() {
  const api = useAuthClient()

  const { data: statsData, isError: statsError } = useQuery({
    queryKey: ['admin-stats'],
    queryFn: () => api.GET('/api/v1/stats').then(r => r.data),
    enabled: true,
  })

  const { data: mcpData } = useQuery({
    queryKey: ['admin-mcp-recent'],
    queryFn: () => api.GET('/api/v1/mcp/servers', { params: { query: { limit: 5 } } }).then(r => r.data),
    enabled: true,
  })

  const { data: agentsData } = useQuery({
    queryKey: ['admin-agents-recent'],
    queryFn: () => api.GET('/api/v1/agents', { params: { query: { limit: 5 } } }).then(r => r.data),
    enabled: true,
  })

  const recentMcp = mcpData?.items ?? []
  const recentAgents = agentsData?.items ?? []

  const stats = [
    { label: 'MCP Servers', value: statsData?.mcp_servers ?? '—', icon: Server, href: '/admin/mcp' },
    { label: 'Agents',      value: statsData?.agents      ?? '—', icon: Bot,    href: '/admin/agents' },
    { label: 'Publishers',  value: statsData?.publishers  ?? '—', icon: Users,  href: '/admin/publishers' },
  ]

  return (
    <div className="space-y-8 max-w-4xl mx-auto">
      <div>
        <h1 className="text-2xl font-bold">Dashboard</h1>
        <p className="text-muted-foreground mt-1">Registry overview.</p>
      </div>

      {statsError && (
        <div
          role="alert"
          className="rounded-md border border-destructive/50 bg-destructive/10 px-4 py-3 text-sm text-destructive"
          data-testid="stats-error"
        >
          Failed to load stats. Check your connection and try again.
        </div>
      )}

      {/* Stat cards */}
      <div className="grid gap-4 sm:grid-cols-3">
        {stats.map(({ label, value, icon: Icon, href }) => (
          <Card key={label}>
            <CardHeader className="pb-2">
              {/* A styled div, not CardTitle (h3): the dashboard's h1 has no
                  intervening h2, so a heading here would skip levels. */}
              <div className="text-sm font-medium text-muted-foreground flex items-center gap-2">
                <Icon className="h-4 w-4" />
                {label}
              </div>
            </CardHeader>
            <CardContent className="flex items-end justify-between">
              <p className="text-3xl font-bold">{value}</p>
              <Button variant="ghost" size="sm" asChild>
                <Link to={href} className="flex items-center gap-1 text-xs">
                  Manage <ArrowRight className="h-3 w-3" />
                </Link>
              </Button>
            </CardContent>
          </Card>
        ))}
      </div>

      {/* Status breakdown */}
      {statsData?.mcp_status_breakdown && statsData?.agent_status_breakdown && (
        <div className="grid gap-4 sm:grid-cols-2">
          {[
            { label: 'MCP Servers', breakdown: statsData.mcp_status_breakdown, href: '/admin/mcp' },
            { label: 'Agents', breakdown: statsData.agent_status_breakdown, href: '/admin/agents' },
          ].map(({ label, breakdown, href }) => (
            <Card key={label}>
              <CardHeader className="pb-2">
                <div className="text-sm font-medium text-muted-foreground">{label} by Status</div>
              </CardHeader>
              <CardContent className="space-y-2">
                <div className="flex items-center justify-between text-sm">
                  <span className="flex items-center gap-1.5">
                    <FileText className="h-3.5 w-3.5 text-muted-foreground" /> Draft
                  </span>
                  <Link to={`${href}?status=draft`}>
                    <Badge variant="outline" className="text-xs">{breakdown.draft}</Badge>
                  </Link>
                </div>
                <div className="flex items-center justify-between text-sm">
                  <span className="flex items-center gap-1.5">
                    <CheckCircle className="h-3.5 w-3.5 text-green-600" /> Published
                  </span>
                  <Link to={`${href}?status=published`}>
                    <Badge variant="outline" className="text-xs">{breakdown.published}</Badge>
                  </Link>
                </div>
                <div className="flex items-center justify-between text-sm">
                  <span className="flex items-center gap-1.5">
                    <AlertTriangle className="h-3.5 w-3.5 text-yellow-600" /> Deprecated
                  </span>
                  <Link to={`${href}?status=deprecated`}>
                    <Badge variant="outline" className="text-xs">{breakdown.deprecated}</Badge>
                  </Link>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      )}

      {/* No "Quick Actions" block: the same New buttons live one click away
          on the list pages the stat cards already link to. */}
      <Separator />

      {/* Recent entries */}
      <div className="grid gap-6 sm:grid-cols-2">
        {/* Recent MCP servers */}
        <div className="space-y-3">
          <div className="flex items-center justify-between">
            <h2 className="text-sm font-semibold text-muted-foreground uppercase tracking-wide">Recent MCP Servers</h2>
            <Button variant="ghost" size="sm" asChild>
              <Link to="/admin/mcp" className="flex items-center gap-1 text-xs">
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
                  to={`/admin/mcp/${s.namespace}/${s.slug}`}
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
              <Link to="/admin/agents" className="flex items-center gap-1 text-xs">
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
                  to={`/admin/agents/${a.namespace}/${a.slug}`}
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
