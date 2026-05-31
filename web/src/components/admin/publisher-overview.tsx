import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { Server, Bot, Users, ClipboardCheck, FilePen, ArrowRight, Activity, Plus } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { EmptyState } from '@/components/ui/empty-state'
import { ActivityTimeline } from '@/components/admin/activity-timeline'
import { useAuthClient } from '@/lib/api-client'
import { useAuth } from '@/auth/AuthContext'
import { usePermissions } from '@/auth/useMe'
import type { PublisherOption } from '@/auth/PublisherContext'
import type { components } from '@/lib/schema'

type StatusBreakdown = components['schemas']['StatusBreakdown']

// StatusBar renders a thin draft/published/deprecated proportion bar under a
// metric. Segments are omitted when zero so a single-status publisher reads as
// one solid colour rather than a near-invisible sliver.
function StatusBar({ b }: { b?: StatusBreakdown }) {
  const draft = b?.draft ?? 0
  const published = b?.published ?? 0
  const deprecated = b?.deprecated ?? 0
  if (draft + published + deprecated === 0) return null
  return (
    <div className="mt-2 flex h-1 overflow-hidden rounded-full" aria-hidden="true">
      {draft > 0 && <span className="bg-muted-foreground/50" style={{ flex: draft }} />}
      {published > 0 && <span className="bg-green-600" style={{ flex: published }} />}
      {deprecated > 0 && <span className="bg-amber-500" style={{ flex: deprecated }} />}
    </div>
  )
}

interface AttentionTile {
  key: string
  show: boolean
  count: number
  label: string
  to: string
  icon: typeof ClipboardCheck
  tone: 'info' | 'warning'
}

const TILE_TONE: Record<AttentionTile['tone'], string> = {
  info: 'bg-blue-50 text-blue-800 hover:bg-blue-100 dark:bg-blue-950/40 dark:text-blue-200 dark:hover:bg-blue-950/70',
  warning:
    'bg-amber-50 text-amber-800 hover:bg-amber-100 dark:bg-amber-950/40 dark:text-amber-200 dark:hover:bg-amber-950/70',
}

// PublisherOverview is the scoped admin home: what needs the caller, the state
// of the publisher, and what happened recently — all for the selected publisher.
export function PublisherOverview({ slug, option }: { slug: string; option: PublisherOption | null }) {
  const { accessToken } = useAuth()
  const api = useAuthClient()
  const perms = usePermissions()

  const { data: stats, isPending: statsPending } = useQuery({
    queryKey: ['publisher-stats', slug],
    queryFn: () =>
      api.GET('/api/v1/publishers/{slug}/stats', { params: { path: { slug } } }).then((r) => r.data),
    enabled: !!accessToken && !!slug,
  })

  const { data: activity, isPending: activityPending } = useQuery({
    queryKey: ['publisher-activity', slug],
    queryFn: () =>
      api
        .GET('/api/v1/publishers/{slug}/activity', { params: { path: { slug }, query: { limit: 8 } } })
        .then((r) => r.data),
    enabled: !!accessToken && !!slug,
  })

  const canEdit = perms.canEdit(slug)
  const canReview = perms.canReview(slug)
  const events = activity?.items ?? []
  const draftCount = (stats?.mcp_status_breakdown?.draft ?? 0) + (stats?.agent_status_breakdown?.draft ?? 0)
  const noResources = !!stats && stats.mcp_servers === 0 && stats.agents === 0

  const tiles: AttentionTile[] = [
    {
      key: 'review',
      show: canReview,
      count: stats?.pending_review ?? 0,
      label: 'awaiting your review',
      to: '/admin/review',
      icon: ClipboardCheck,
      tone: 'info',
    },
    {
      key: 'drafts',
      show: canEdit,
      count: draftCount,
      label: draftCount === 1 ? 'draft in progress' : 'drafts in progress',
      to: '/admin/mcp?status=draft',
      icon: FilePen,
      tone: 'warning',
    },
  ]
  const visibleTiles = tiles.filter((t) => t.show && t.count > 0)

  return (
    <div className="space-y-8 max-w-4xl mx-auto">
      {/* Header */}
      <div className="flex flex-wrap items-center gap-3">
        <div className="min-w-0 flex-1">
          <h1 className="text-2xl font-bold truncate">{option?.name ?? slug}</h1>
          <p className="text-muted-foreground mt-1 text-sm">
            Everything happening in <span className="font-mono">{slug}</span>.
          </p>
        </div>
        <div className="flex flex-wrap gap-1.5">
          {option?.roles.map((r) => (
            <Badge key={r} variant="outline" className="capitalize">
              {r}
            </Badge>
          ))}
        </div>
      </div>

      {/* Attention strip — only the items that actually need the caller. */}
      {visibleTiles.length > 0 && (
        <div className="space-y-2">
          <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">Needs you</p>
          <div className="flex flex-wrap gap-3">
            {visibleTiles.map(({ key, count, label, to, icon: Icon, tone }) => (
              <Link
                key={key}
                to={to}
                className={`flex flex-1 items-center gap-3 rounded-lg px-4 py-3 transition-colors min-w-[220px] ${TILE_TONE[tone]}`}
              >
                <Icon className="h-5 w-5 shrink-0" aria-hidden="true" />
                <span className="flex-1 text-sm">
                  <span className="text-lg font-bold">{count}</span> {label}
                </span>
                <ArrowRight className="h-4 w-4 shrink-0" aria-hidden="true" />
              </Link>
            ))}
          </div>
        </div>
      )}

      {/* Metric row */}
      {statsPending ? (
        <div className="grid gap-4 sm:grid-cols-3">
          {[0, 1, 2].map((i) => (
            <Skeleton key={i} className="h-24 rounded-lg" />
          ))}
        </div>
      ) : noResources ? (
        <EmptyState
          icon={<Server className="h-10 w-10" />}
          title="No resources yet"
          description={
            canEdit
              ? 'Publish your first MCP server or agent to this publisher.'
              : 'This publisher has no MCP servers or agents yet.'
          }
          action={
            canEdit ? (
              <Button asChild size="sm">
                <Link to="/admin/mcp/new" className="flex items-center gap-1.5">
                  <Plus className="h-4 w-4" /> New MCP Server
                </Link>
              </Button>
            ) : undefined
          }
        />
      ) : (
        stats && (
          <div className="grid gap-4 sm:grid-cols-3">
            <Link to="/admin/mcp" className="rounded-lg bg-muted/50 p-4 transition-colors hover:bg-muted">
              <p className="flex items-center gap-1.5 text-xs text-muted-foreground">
                <Server className="h-3.5 w-3.5" aria-hidden="true" /> MCP servers
              </p>
              <p className="mt-1 text-2xl font-bold">{stats.mcp_servers}</p>
              <StatusBar b={stats.mcp_status_breakdown} />
            </Link>
            <Link to="/admin/agents" className="rounded-lg bg-muted/50 p-4 transition-colors hover:bg-muted">
              <p className="flex items-center gap-1.5 text-xs text-muted-foreground">
                <Bot className="h-3.5 w-3.5" aria-hidden="true" /> Agents
              </p>
              <p className="mt-1 text-2xl font-bold">{stats.agents}</p>
              <StatusBar b={stats.agent_status_breakdown} />
            </Link>
            <div className="rounded-lg bg-muted/50 p-4">
              <p className="flex items-center gap-1.5 text-xs text-muted-foreground">
                <Users className="h-3.5 w-3.5" aria-hidden="true" /> Members
              </p>
              <p className="mt-1 text-2xl font-bold">{stats.members}</p>
              {stats.member_roles && (
                <p className="mt-2 text-[11px] text-muted-foreground">
                  {stats.member_roles.editors} editor{stats.member_roles.editors === 1 ? '' : 's'} ·{' '}
                  {stats.member_roles.reviewers} reviewer{stats.member_roles.reviewers === 1 ? '' : 's'} ·{' '}
                  {stats.member_roles.viewers} viewer{stats.member_roles.viewers === 1 ? '' : 's'}
                </p>
              )}
            </div>
          </div>
        )
      )}

      {/* Recent activity */}
      {!noResources && (
        <div className="space-y-3">
          <h2 className="text-lg font-semibold">Recent activity</h2>
          {activityPending ? (
            <div className="space-y-3">
              {[0, 1, 2].map((i) => (
                <Skeleton key={i} className="h-10 rounded-md" />
              ))}
            </div>
          ) : events.length === 0 ? (
            <EmptyState
              icon={<Activity className="h-8 w-8" />}
              title="No activity yet"
              description="Authoring, reviews, and visibility changes will show up here."
            />
          ) : (
            <ActivityTimeline events={events} />
          )}
        </div>
      )}
    </div>
  )
}
