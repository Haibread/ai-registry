import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { Plug, ClipboardCheck, FilePen, Activity, Plus } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { EmptyState } from '@/components/ui/empty-state'
import { ActivityTimeline } from '@/components/admin/activity-timeline'
import { AttentionTiles, type AttentionTile } from '@/components/admin/attention-tiles'
import { useAuthClient } from '@/lib/api-client'
import { usePermissions } from '@/auth/useMe'
import type { PublisherOption } from '@/auth/PublisherContext'
import type { components } from '@/lib/schema'

type StatusBreakdown = components['schemas']['StatusBreakdown']

// StatusBar renders a thin draft/published/deprecated proportion bar under a
// metric, with a text caption beneath it. Zero segments are omitted so a
// single-status publisher reads as one solid colour rather than a near-invisible
// sliver; the caption (e.g. "9 published · 2 draft") gives the bar a legible,
// screen-reader-accessible meaning, so the bar itself stays decorative.
function StatusBar({ b }: { b?: StatusBreakdown }) {
  const segments = [
    { key: 'draft', count: b?.draft ?? 0, color: 'bg-muted-foreground/50' },
    { key: 'published', count: b?.published ?? 0, color: 'bg-success' },
    { key: 'deprecated', count: b?.deprecated ?? 0, color: 'bg-warning' },
  ].filter((s) => s.count > 0)
  if (segments.length === 0) return null
  return (
    <>
      <div className="mt-2 flex h-1 overflow-hidden rounded-full" aria-hidden="true">
        {segments.map((s) => (
          <span key={s.key} className={s.color} style={{ flex: s.count }} />
        ))}
      </div>
      <p className="mt-1.5 text-[11px] text-muted-foreground">
        {segments.map((s) => `${s.count} ${s.key}`).join(' · ')}
      </p>
    </>
  )
}

// PublisherOverview is the scoped admin home: what needs the caller, the state
// of the publisher, and what happened recently — all for the selected publisher.
export function PublisherOverview({ slug, option }: { slug: string; option: PublisherOption | null }) {
  const api = useAuthClient()
  const perms = usePermissions()

  const { data: stats, isPending: statsPending } = useQuery({
    queryKey: ['publisher-stats', slug],
    queryFn: () =>
      api.GET('/api/v1/publishers/{slug}/stats', { params: { path: { slug } } }).then((r) => r.data),
    enabled: !!slug,
  })

  const { data: activity, isPending: activityPending } = useQuery({
    queryKey: ['publisher-activity', slug],
    queryFn: () =>
      api
        .GET('/api/v1/publishers/{slug}/activity', { params: { path: { slug }, query: { limit: 8 } } })
        .then((r) => r.data),
    enabled: !!slug,
  })

  const canEdit = perms.canEdit(slug)
  const canReview = perms.canReview(slug)
  const events = activity?.items ?? []
  const draftCount = stats?.mcp_status_breakdown?.draft ?? 0
  const noResources = !!stats && stats.mcp_servers === 0

  const tiles: AttentionTile[] = [
    ...(canReview ? [{
      key: 'review',
      count: stats?.pending_review ?? 0,
      label: 'awaiting your review',
      to: '/admin/review',
      icon: ClipboardCheck,
      tone: 'attention' as const,
    }] : []),
    ...(canEdit ? [{
      key: 'drafts',
      count: draftCount,
      label: draftCount === 1 ? 'draft in progress' : 'drafts in progress',
      to: '/admin/mcp?status=draft',
      icon: FilePen,
      tone: 'neutral' as const,
    }] : []),
  ]

  return (
    <div className="space-y-8 max-w-4xl mx-auto">
      {/* Header */}
      <div className="flex flex-wrap items-center gap-3">
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-2">
            <h1 className="text-2xl font-bold truncate">{option?.name ?? slug}</h1>
            {option?.roles.map((r) => (
              <Badge key={r} variant="outline" className="capitalize">
                {r}
              </Badge>
            ))}
          </div>
          <p className="text-muted-foreground mt-1 text-sm">Everything happening in {option?.name ?? slug}.</p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          {canEdit && (
            <Button asChild size="sm">
              <Link to="/admin/mcp/new" className="flex items-center gap-1.5">
                <Plus className="h-4 w-4" aria-hidden="true" /> New MCP server
              </Link>
            </Button>
          )}
        </div>
      </div>

      <AttentionTiles tiles={tiles} />

      {/* Metric row */}
      {statsPending ? (
        <div className="grid gap-4 sm:grid-cols-2">
          <Skeleton className="h-24 rounded-lg" />
        </div>
      ) : noResources ? (
        <EmptyState
          icon={<Plug className="h-10 w-10" />}
          title="No resources yet"
          description={
            canEdit
              ? 'Publish your first MCP server to this publisher.'
              : 'This publisher has no MCP servers yet.'
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
          <div className="grid gap-4 sm:grid-cols-2">
            <Link to="/admin/mcp" className="rounded-lg bg-muted/50 p-4 transition-colors hover:bg-muted">
              <p className="flex items-center gap-1.5 text-xs text-muted-foreground">
                <Plug className="h-3.5 w-3.5" aria-hidden="true" /> MCP servers
              </p>
              <p className="mt-1 text-2xl font-bold">{stats.mcp_servers}</p>
              <StatusBar b={stats.mcp_status_breakdown} />
            </Link>
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
