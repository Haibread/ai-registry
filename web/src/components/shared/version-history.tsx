/**
 * VersionHistory — timeline list of all published versions for an entry.
 *
 * Fetches the versions endpoint and renders a compact timeline. Includes an
 * opt-in compare mode: reviewers click "Compare versions" to enter selection
 * mode, pick two rows, and see a side-by-side structural diff below the list.
 */

import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { VersionDiff } from './version-diff'
import { getPublicClient } from '@/lib/api-client'
import { formatDate } from '@/lib/utils'

/**
 * VersionLike is the structural shape this component actually reads.
 * Both MCPServerVersion and AgentVersion satisfy it; using a local
 * structural type avoids dragging full schema unions through the view.
 */
interface VersionLike {
  id?: string
  version: string
  status?: string
  published_at?: string
  // Extra fields are passed through to VersionDiff unchanged.
  [key: string]: unknown
}

interface VersionHistoryProps {
  type: 'mcp' | 'agent'
  namespace: string
  slug: string
  latestVersion?: string
}

export function VersionHistory({ type, namespace, slug, latestVersion }: VersionHistoryProps) {
  const api = getPublicClient()

  const { data, isLoading } = useQuery({
    queryKey: ['versions', type, namespace, slug],
    queryFn: async () => {
      if (type === 'mcp') {
        const r = await api.GET('/api/v1/mcp/servers/{namespace}/{slug}/versions', {
          params: { path: { namespace, slug } },
        })
        return r.data
      }
      const r = await api.GET('/api/v1/agents/{namespace}/{slug}/versions', {
        params: { path: { namespace, slug } },
      })
      return r.data
    },
  })

  if (isLoading) {
    return (
      <div className="space-y-3">
        {Array.from({ length: 3 }).map((_, i) => (
          <div key={i} className="flex gap-3 items-center">
            <Skeleton className="h-4 w-16 rounded" />
            <Skeleton className="h-3 w-24 rounded" />
          </div>
        ))}
      </div>
    )
  }

  // The openapi-typescript generated union for versions is a heavy discriminated
  // shape; normalize to either a bare array or { items: [] } and treat the rows
  // structurally as VersionLike.
  const raw = data as VersionLike[] | { items?: VersionLike[] } | undefined
  const versions: VersionLike[] = Array.isArray(raw)
    ? raw
    : Array.isArray(raw?.items)
      ? raw.items
      : []

  return <VersionHistoryView versions={versions} latestVersion={latestVersion} />
}

interface VersionHistoryViewProps {
  versions: VersionLike[]
  latestVersion?: string
}

export function VersionHistoryView({ versions, latestVersion }: VersionHistoryViewProps) {
  const [compareMode, setCompareMode] = useState(false)
  const [selected, setSelected] = useState<string[]>([])

  if (versions.length === 0) {
    return <p className="text-sm text-muted-foreground">No versions published yet.</p>
  }

  function toggleSelected(id: string) {
    setSelected((prev) => {
      if (prev.includes(id)) return prev.filter((x) => x !== id)
      if (prev.length >= 2) return [prev[1], id]
      return [...prev, id]
    })
  }

  const canCompare = versions.length >= 2
  const selectedVersions: VersionLike[] = selected
    .map((id) => versions.find((v) => String(v.id ?? v.version) === id))
    .filter((v): v is VersionLike => v !== undefined)

  return (
    <div className="space-y-4">
      {canCompare && (
        <div className="flex items-center gap-2">
          <Button
            type="button"
            variant={compareMode ? 'default' : 'outline'}
            size="sm"
            onClick={() => {
              setCompareMode((m) => !m)
              setSelected([])
            }}
          >
            {compareMode ? 'Exit compare' : 'Compare versions'}
          </Button>
          {compareMode && (
            <span className="text-xs text-muted-foreground">
              Select two versions to diff ({selected.length}/2)
            </span>
          )}
        </div>
      )}

      <div className="space-y-0">
        {versions.map((v, i) => {
          const isLatest = v.version === latestVersion
          const isPublished = !!v.published_at
          const key = String(v.id ?? v.version ?? i)
          const isSelected = selected.includes(key)
          const rowProps = compareMode
            ? {
                role: 'button' as const,
                tabIndex: 0,
                onClick: () => toggleSelected(key),
                onKeyDown: (e: React.KeyboardEvent) => {
                  if (e.key === 'Enter' || e.key === ' ') {
                    e.preventDefault()
                    toggleSelected(key)
                  }
                },
              }
            : {}
          return (
            <div
              key={v.id ?? i}
              {...rowProps}
              className={`flex items-center gap-3 py-2 border-l-2 pl-4 relative transition-colors ${
                compareMode ? 'cursor-pointer hover:bg-accent/40 rounded-sm' : ''
              } ${isSelected ? 'bg-accent/60' : ''}`}
              style={{
                borderColor: isPublished ? 'var(--color-primary)' : 'var(--color-border)',
              }}
            >
              {/* Timeline dot */}
              <div
                className={`absolute -left-[5px] h-2 w-2 rounded-full ${
                  isPublished ? 'bg-primary' : 'bg-muted-foreground/30'
                }`}
              />

              {compareMode && (
                <input
                  type="checkbox"
                  checked={isSelected}
                  onChange={() => toggleSelected(key)}
                  onClick={(e) => e.stopPropagation()}
                  aria-label={`Select version ${v.version} for comparison`}
                  className="h-3.5 w-3.5"
                />
              )}

              <Badge variant="outline" className="font-mono text-xs shrink-0">
                v{v.version}
              </Badge>

              {isLatest && (
                <Badge variant="default" className="text-[10px] px-1.5 py-0">
                  Latest
                </Badge>
              )}

              {v.status && v.status !== 'active' && (
                <Badge variant="muted" className="text-[10px] px-1.5 py-0">
                  {v.status}
                </Badge>
              )}

              <span className="text-xs text-muted-foreground">
                {v.published_at ? formatDate(v.published_at) : 'Draft'}
              </span>
            </div>
          )
        })}
      </div>

      {compareMode && selectedVersions.length === 2 && (
        <VersionDiff a={selectedVersions[0]} b={selectedVersions[1]} />
      )}
    </div>
  )
}
