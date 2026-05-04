/**
 * Admin Audit page — full-fidelity view of the audit log.
 *
 * Unlike the public per-entry /activity feed (privacy-scrubbed, per-resource),
 * this page shows EVERY recorded mutation across the registry with complete
 * actor identity and metadata. Admin eyes only — surfaced via the admin-
 * guarded /api/v1/audit endpoint.
 *
 * Information density is intentional: filters on resource type, action, and
 * actor; expandable rows that reveal raw metadata JSON; drill-down links to
 * the admin detail pages for each mutated resource.
 */

import { useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import {
  Activity,
  Filter,
  ChevronDown,
  ChevronRight,
  RotateCcw,
  Server,
  Bot,
  Users,
  ExternalLink,
} from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { EmptyState } from '@/components/ui/empty-state'
import { Skeleton } from '@/components/ui/skeleton'
import { useAuthClient } from '@/lib/api-client'
import { useAuth } from '@/auth/AuthContext'
import type { components } from '@/lib/schema'

type AuditEvent = components['schemas']['AuditEvent']

// ─── Helpers ────────────────────────────────────────────────────────────────

/** All known audit actions grouped by resource type, for the filter Select. */
const ACTIONS_BY_TYPE: Record<string, string[]> = {
  mcp_server: [
    'mcp_server.created',
    'mcp_server.updated',
    'mcp_server.deleted',
    'mcp_server.deprecated',
    'mcp_server.visibility_changed',
    'mcp_server_version.created',
    'mcp_server_version.published',
  ],
  agent: [
    'agent.created',
    'agent.updated',
    'agent.deleted',
    'agent.deprecated',
    'agent.visibility_changed',
    'agent_version.created',
    'agent_version.published',
  ],
  publisher: [
    'publisher.created',
    'publisher.updated',
    'publisher.deleted',
  ],
}

/**
 * resourceTypeIcon returns the sidebar-nav icon that matches the resource
 * type, so rows visually connect to the admin section a reviewer would
 * drill into.
 */
function resourceTypeIcon(t: string) {
  if (t === 'mcp_server') return Server
  if (t === 'agent') return Bot
  if (t === 'publisher') return Users
  return Activity
}

/** badgeForAction picks a variant that telegraphs severity at a glance. */
function badgeVariantForAction(
  action: string,
): 'default' | 'secondary' | 'destructive' | 'muted' | 'outline' {
  if (action.endsWith('.deleted')) return 'destructive'
  if (action.endsWith('.deprecated')) return 'destructive'
  if (action.endsWith('.published')) return 'default'
  if (action.endsWith('.created')) return 'default'
  if (action.endsWith('.updated')) return 'secondary'
  if (action.endsWith('.visibility_changed')) return 'secondary'
  return 'outline'
}

/**
 * drillDownHref returns the admin detail URL for the mutated resource.
 * Publisher and resource detail pages both live under /admin/{type}/{slug}.
 */
function drillDownHref(e: AuditEvent): string | null {
  if (!e.resource_ns && !e.resource_slug) return null
  if (e.resource_type === 'mcp_server' && e.resource_ns && e.resource_slug) {
    return `/admin/mcp/${e.resource_ns}/${e.resource_slug}`
  }
  if (e.resource_type === 'agent' && e.resource_ns && e.resource_slug) {
    return `/admin/agents/${e.resource_ns}/${e.resource_slug}`
  }
  if (e.resource_type === 'publisher' && e.resource_slug) {
    return `/admin/publishers/${e.resource_slug}`
  }
  return null
}

function formatFullTimestamp(iso: string): string {
  return new Date(iso).toLocaleString(undefined, {
    year: 'numeric',
    month: 'short',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  })
}

// ─── Component ──────────────────────────────────────────────────────────────

export default function AdminAuditPage() {
  const { accessToken } = useAuth()
  const api = useAuthClient()

  const [resourceType, setResourceType] = useState<string>('all')
  const [action, setAction] = useState<string>('all')
  const [actor, setActor] = useState<string>('')
  const [pages, setPages] = useState<AuditEvent[][]>([])
  const [cursor, setCursor] = useState<string | undefined>(undefined)
  const [expandedId, setExpandedId] = useState<string | null>(null)

  // React-query key encodes all filters + the current cursor, so a filter
  // change triggers a refetch and a cursor advance triggers the next page.
  const queryKey = ['admin-audit', resourceType, action, actor, cursor ?? null]

  const { data, isLoading, isError, refetch, isFetching } = useQuery({
    queryKey,
    queryFn: async () => {
      const query: Record<string, string> = {}
      if (resourceType !== 'all') query.resource_type = resourceType
      // The backend supports filtering by resource_type but not action —
      // filter client-side for `action`. We still pass resource_type server-
      // side so we don't pull 100 unrelated rows to locally filter.
      if (actor.trim()) query.actor = actor.trim()
      if (cursor) query.cursor = cursor
      query.limit = '50'
      const r = await api.GET('/api/v1/audit', { params: { query } })
      return r.data ?? null
    },
    enabled: !!accessToken,
    // Keep previous page visible while the next one loads.
    placeholderData: (prev) => prev,
  })

  // Accumulate pages. When filters change (cursor reset to undefined), the
  // accumulator resets in the filter reset handler below, not here.
  const thisPage = useMemo(() => data?.items ?? [], [data])
  const allRows = useMemo(() => {
    if (cursor === undefined) return thisPage
    return [...pages.flat(), ...thisPage]
  }, [cursor, pages, thisPage])

  // Client-side action filter (server supports only resource_type + actor).
  const filtered = useMemo(() => {
    if (action === 'all') return allRows
    return allRows.filter((e) => e.action === action)
  }, [allRows, action])

  const canLoadMore = !!data?.next_cursor

  function resetFilters() {
    setResourceType('all')
    setAction('all')
    setActor('')
    setPages([])
    setCursor(undefined)
  }

  function loadMore() {
    if (!data?.next_cursor) return
    // Snapshot the page we've just consumed so the accumulator keeps
    // growing as the user walks the log.
    setPages((p) => [...p, thisPage])
    setCursor(data.next_cursor)
  }

  // Available action options — narrow by resource_type when one is chosen.
  const actionOptions = useMemo(() => {
    if (resourceType !== 'all' && ACTIONS_BY_TYPE[resourceType]) {
      return ACTIONS_BY_TYPE[resourceType]
    }
    return Object.values(ACTIONS_BY_TYPE).flat()
  }, [resourceType])

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-3">
        <Activity className="h-6 w-6 text-muted-foreground" />
        <h1 className="text-2xl font-bold">Activity</h1>
        <Badge variant="outline" className="text-xs">Admin</Badge>
      </div>
      <p className="text-sm text-muted-foreground max-w-3xl">
        Every recorded mutation across the registry — creation, updates,
        publish, deprecate, visibility, and delete events. Includes actor
        identity and raw metadata for auditability. The public per-entry
        feed on detail pages surfaces a privacy-scrubbed subset of the same
        data.
      </p>

      {/* ─── Filters ─────────────────────────────────────────────────────── */}
      <div className="rounded-md border p-3 space-y-3 bg-muted/30">
        <div className="flex items-center gap-2 text-sm font-medium">
          <Filter className="h-4 w-4" />
          Filters
        </div>
        <div className="grid gap-3 sm:grid-cols-2 md:grid-cols-4">
          <div className="space-y-1">
            <label className="text-xs text-muted-foreground">Resource type</label>
            <Select
              value={resourceType}
              onValueChange={(v) => {
                setResourceType(v)
                // Reset action to "all" so an incompatible choice doesn't
                // silently filter everything out.
                setAction('all')
                setPages([])
                setCursor(undefined)
              }}
            >
              <SelectTrigger aria-label="Resource type">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">All types</SelectItem>
                <SelectItem value="mcp_server">MCP servers</SelectItem>
                <SelectItem value="agent">Agents</SelectItem>
                <SelectItem value="publisher">Publishers</SelectItem>
              </SelectContent>
            </Select>
          </div>

          <div className="space-y-1">
            <label className="text-xs text-muted-foreground">Action</label>
            <Select
              value={action}
              onValueChange={(v) => {
                setAction(v)
              }}
            >
              <SelectTrigger aria-label="Action">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">All actions</SelectItem>
                {actionOptions.map((a) => (
                  <SelectItem key={a} value={a}>
                    {a}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className="space-y-1 md:col-span-2">
            <label className="text-xs text-muted-foreground">
              Actor (Keycloak subject UUID)
            </label>
            <Input
              value={actor}
              onChange={(e) => setActor(e.target.value)}
              placeholder="e.g. a1b2c3d4-…"
              aria-label="Actor subject"
            />
          </div>
        </div>
        <div className="flex items-center gap-2 pt-1">
          <Button
            type="button"
            size="sm"
            variant="outline"
            onClick={() => {
              setPages([])
              setCursor(undefined)
              refetch()
            }}
          >
            Apply
          </Button>
          <Button
            type="button"
            size="sm"
            variant="ghost"
            onClick={resetFilters}
          >
            <RotateCcw className="h-3.5 w-3.5" />
            <span className="ml-1.5">Reset</span>
          </Button>
          <span className="ml-auto text-xs text-muted-foreground">
            {filtered.length} event{filtered.length === 1 ? '' : 's'} shown
            {canLoadMore && ' (more available)'}
          </span>
        </div>
      </div>

      {/* ─── Body ─────────────────────────────────────────────────────────── */}
      {isLoading && allRows.length === 0 ? (
        <div className="space-y-2" data-testid="audit-loading">
          {Array.from({ length: 5 }).map((_, i) => (
            <Skeleton key={i} className="h-16 w-full rounded-md" />
          ))}
        </div>
      ) : isError ? (
        <p className="text-sm text-destructive">Failed to load audit events.</p>
      ) : filtered.length === 0 ? (
        <EmptyState
          icon={<Activity className="h-10 w-10" />}
          title="No audit events match"
          description="Adjust filters or clear them to see the full log."
        />
      ) : (
        <ul className="divide-y rounded-md border" data-testid="audit-rows">
          {filtered.map((e) => {
            const ResourceIcon = resourceTypeIcon(e.resource_type)
            const href = drillDownHref(e)
            const expanded = expandedId === e.id
            const hasMetadata =
              e.metadata && Object.keys(e.metadata).length > 0
            return (
              <li key={e.id} className="px-3 py-2.5">
                <div className="flex items-start gap-2 flex-wrap">
                  <button
                    type="button"
                    onClick={() =>
                      setExpandedId(expanded ? null : e.id)
                    }
                    className="mt-0.5 text-muted-foreground hover:text-foreground shrink-0"
                    aria-label={expanded ? 'Collapse row' : 'Expand row'}
                    aria-expanded={expanded}
                  >
                    {expanded ? (
                      <ChevronDown className="h-4 w-4" />
                    ) : (
                      <ChevronRight className="h-4 w-4" />
                    )}
                  </button>
                  <ResourceIcon className="h-4 w-4 mt-1 text-muted-foreground shrink-0" />
                  <Badge
                    variant={badgeVariantForAction(e.action)}
                    className="text-xs font-mono"
                  >
                    {e.action}
                  </Badge>
                  {href && (e.resource_ns || e.resource_slug) && (
                    <a
                      href={href}
                      className="text-xs font-mono text-primary hover:underline inline-flex items-center gap-1"
                    >
                      {e.resource_ns ? `${e.resource_ns}/` : ''}
                      {e.resource_slug}
                      <ExternalLink className="h-3 w-3" />
                    </a>
                  )}
                  <span className="text-xs text-muted-foreground ml-auto tabular-nums">
                    {formatFullTimestamp(e.created_at)}
                  </span>
                </div>

                <div className="mt-1 ml-8 flex flex-wrap gap-x-4 gap-y-1 text-xs text-muted-foreground">
                  <span>
                    <span className="uppercase tracking-wide mr-1">Actor:</span>
                    <span className="font-mono text-foreground">
                      {e.actor_email || '(unknown)'}
                    </span>
                  </span>
                  <span>
                    <span className="uppercase tracking-wide mr-1">Subject:</span>
                    <span className="font-mono">
                      {e.actor_subject || '(unknown)'}
                    </span>
                  </span>
                  <span>
                    <span className="uppercase tracking-wide mr-1">ID:</span>
                    <span className="font-mono">{e.id}</span>
                  </span>
                  <span>
                    <span className="uppercase tracking-wide mr-1">Resource ID:</span>
                    <span className="font-mono">{e.resource_id}</span>
                  </span>
                </div>

                {expanded && hasMetadata && (
                  <div className="mt-2 ml-8">
                    <div className="text-xs text-muted-foreground uppercase tracking-wide mb-1">
                      Metadata
                    </div>
                    <pre className="rounded bg-muted/60 p-2 text-xs font-mono overflow-x-auto">
                      {JSON.stringify(e.metadata, null, 2)}
                    </pre>
                  </div>
                )}
                {expanded && !hasMetadata && (
                  <p className="mt-2 ml-8 text-xs text-muted-foreground italic">
                    No metadata recorded for this event.
                  </p>
                )}
              </li>
            )
          })}
        </ul>
      )}

      {canLoadMore && (
        <div className="pt-1">
          <Button
            type="button"
            size="sm"
            variant="outline"
            onClick={loadMore}
            disabled={isFetching}
          >
            {isFetching ? 'Loading…' : 'Load more'}
          </Button>
        </div>
      )}
    </div>
  )
}
