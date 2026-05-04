/**
 * ActivityFeed — privacy-scrubbed lifecycle feed for a registry entry.
 *
 * Renders the events returned by the public `/activity` endpoints. Actor
 * identity is never exposed by the backend; this component reinforces that
 * contract by only displaying fields the server chose to expose.
 *
 * The feed supports cursor pagination via a "Load more" button rather than
 * auto-scrolling: per-entry activity is historic context, not a live
 * timeline, so controlling when more data is fetched keeps the page cheap.
 */

import { useEffect, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import {
  Sparkles,
  Upload,
  Ban,
  EyeOff,
  Eye,
  Pencil,
  CircleDot,
} from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { SectionHeader } from './section-header'
import { getPublicClient } from '@/lib/api-client'
import type { components } from '@/lib/schema'

type PublicActivityEvent = components['schemas']['PublicActivityEvent']
type PublicActivityEventList = components['schemas']['PublicActivityEventList']

interface ActivityFeedProps {
  resourceType: 'mcp' | 'agent'
  namespace?: string
  slug?: string
  /** Page size. Defaults to 10 to keep the initial render compact. */
  pageSize?: number
}

/**
 * labelFor maps an action key to a short human-readable phrase + an icon.
 * Kept as a table rather than a switch so it's easy to audit which keys
 * are actually rendered — anything not in the table falls through to a
 * sensible default.
 */
function labelFor(
  action: string,
): { label: string; Icon: typeof CircleDot } {
  switch (action) {
    case 'mcp_server.created':
      return { label: 'Server created', Icon: Sparkles }
    case 'agent.created':
      return { label: 'Agent created', Icon: Sparkles }
    case 'mcp_server_version.published':
      return { label: 'Version published', Icon: Upload }
    case 'agent_version.published':
      return { label: 'Version published', Icon: Upload }
    case 'mcp_server.deprecated':
    case 'agent.deprecated':
      return { label: 'Deprecated', Icon: Ban }
    case 'mcp_server.visibility_changed':
    case 'agent.visibility_changed':
      return { label: 'Visibility changed', Icon: EyeOff }
    case 'mcp_server.updated':
    case 'agent.updated':
      return { label: 'Metadata updated', Icon: Pencil }
    default:
      return { label: action, Icon: CircleDot }
  }
}

/**
 * formatWhen returns a compact relative timestamp ("3h ago", "2d ago") with
 * the absolute timestamp as a tooltip. Kept local to this component because
 * the rest of the app uses absolute dates elsewhere.
 */
function formatWhen(iso: string): { short: string; full: string } {
  const t = new Date(iso)
  const full = t.toLocaleString()
  const diffMs = Date.now() - t.getTime()
  const mins = Math.round(diffMs / 60_000)
  if (mins < 1) return { short: 'just now', full }
  if (mins < 60) return { short: `${mins}m ago`, full }
  const hrs = Math.round(mins / 60)
  if (hrs < 24) return { short: `${hrs}h ago`, full }
  const days = Math.round(hrs / 24)
  if (days < 30) return { short: `${days}d ago`, full }
  const months = Math.round(days / 30)
  if (months < 12) return { short: `${months}mo ago`, full }
  const years = Math.round(months / 12)
  return { short: `${years}y ago`, full }
}

/**
 * metadataSummary renders the scrubbed metadata object as a compact
 * "k: v, k: v" string. Values are expected to be primitives because
 * the server drops anything else.
 */
function metadataSummary(md: Record<string, unknown> | undefined): string {
  if (!md) return ''
  const parts: string[] = []
  // Stable key ordering for a predictable display.
  for (const k of ['from', 'to', 'visibility', 'reason', 'field'] as const) {
    const v = md[k]
    if (v === undefined || v === null || v === '') continue
    parts.push(`${k}: ${String(v)}`)
  }
  return parts.join(' · ')
}

export function ActivityFeed({
  resourceType,
  namespace,
  slug,
  pageSize = 10,
}: ActivityFeedProps) {
  const api = getPublicClient()
  const [cursor, setCursor] = useState<string | undefined>(undefined)
  const [accumulated, setAccumulated] = useState<PublicActivityEvent[]>([])
  // Track whether the user has clicked Load-more so we can scope the loading
  // indicator to the button rather than redrawing the whole section.
  const [appending, setAppending] = useState(false)

  const enabled = !!namespace && !!slug

  const { data, isLoading } = useQuery<PublicActivityEventList | null>({
    queryKey: ['activity', resourceType, namespace, slug, cursor ?? null, pageSize],
    queryFn: async () => {
      if (!enabled) return null
      if (resourceType === 'mcp') {
        const r = await api.GET(
          '/api/v1/mcp/servers/{namespace}/{slug}/activity',
          {
            params: {
              path: { namespace: namespace!, slug: slug! },
              query: { limit: pageSize, cursor },
            },
          },
        )
        return r.data ?? null
      }
      const r = await api.GET('/api/v1/agents/{namespace}/{slug}/activity', {
        params: {
          path: { namespace: namespace!, slug: slug! },
          query: { limit: pageSize, cursor },
        },
      })
      return r.data ?? null
    },
    enabled,
  })

  // Merge newly-fetched pages onto the accumulator. `cursor` is part of the
  // query key, so `data` is always page-specific. When `cursor` is undefined
  // (first page) we replace; otherwise we append. useEffect is used — not a
  // render-time setState — to keep this a pure React pattern.
  useEffect(() => {
    if (!data) return
    const newItems = data.items ?? []
    if (cursor === undefined) {
      setAccumulated(newItems)
    } else {
      setAccumulated((prev) => [...prev, ...newItems])
      setAppending(false)
    }
  }, [data, cursor])

  const items = accumulated
  const canLoadMore = !!data?.next_cursor

  if (isLoading && items.length === 0) {
    return (
      <section className="space-y-3">
        <SectionHeader title="Activity" />
        <div className="space-y-2">
          {Array.from({ length: 3 }).map((_, i) => (
            <div key={i} className="flex gap-3 items-center">
              <Skeleton className="h-4 w-4 rounded-full" />
              <Skeleton className="h-3 w-56 rounded" />
            </div>
          ))}
        </div>
      </section>
    )
  }

  if (items.length === 0) {
    return (
      <section className="space-y-2">
        <SectionHeader title="Activity" />
        <p className="text-sm text-muted-foreground">
          No recorded activity yet.
        </p>
      </section>
    )
  }

  return (
    <section className="space-y-3">
      <SectionHeader title="Activity" />
      <ul
        className="space-y-0"
        aria-label="Activity history"
        data-testid="activity-feed"
      >
        {items.map((ev) => {
          const { label, Icon } = labelFor(ev.action)
          const { short, full } = formatWhen(ev.created_at)
          const summary = metadataSummary(
            ev.metadata as Record<string, unknown> | undefined,
          )
          // The `actor_role` and `actor_publisher` labels are the only
          // actor information the API exposes. We surface them as a subtle
          // badge so users can distinguish self-service vs registry-admin
          // actions, but never who specifically acted.
          const role =
            ev.actor_role === 'publisher' ? 'Publisher' : 'Admin'
          return (
            <li
              key={ev.id}
              className="relative flex items-start gap-3 py-2 border-l-2 border-border pl-4"
            >
              <div
                className="absolute -left-[5px] top-4 h-2 w-2 rounded-full bg-muted-foreground/60"
                aria-hidden="true"
              />
              <Icon
                className="h-4 w-4 mt-0.5 text-muted-foreground shrink-0"
                aria-hidden="true"
              />
              <div className="min-w-0 flex-1">
                <div className="flex items-center gap-2 flex-wrap">
                  <span className="text-sm font-medium">{label}</span>
                  {ev.version && (
                    <Badge
                      variant="outline"
                      className="text-[10px] font-mono px-1.5 py-0"
                    >
                      v{ev.version}
                    </Badge>
                  )}
                  <Badge
                    variant="muted"
                    className="text-[10px] px-1.5 py-0"
                    aria-label={`Performed by ${role}`}
                  >
                    <Eye className="h-2.5 w-2.5 mr-1" aria-hidden="true" />
                    {role}
                  </Badge>
                </div>
                {summary && (
                  <p className="text-xs text-muted-foreground truncate mt-0.5">
                    {summary}
                  </p>
                )}
              </div>
              <time
                dateTime={ev.created_at}
                title={full}
                className="text-xs text-muted-foreground shrink-0 tabular-nums mt-1"
              >
                {short}
              </time>
            </li>
          )
        })}
      </ul>

      {canLoadMore && (
        <div className="pt-1">
          <Button
            type="button"
            variant="outline"
            size="sm"
            disabled={appending}
            onClick={() => {
              setAppending(true)
              setCursor(data!.next_cursor)
            }}
          >
            {appending ? 'Loading…' : 'Load more'}
          </Button>
        </div>
      )}
    </section>
  )
}
