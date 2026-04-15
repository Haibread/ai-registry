/**
 * useRecordEvent — fire-and-forget view/copy tracking hooks.
 *
 * The POST requests are non-blocking and errors are silently swallowed.
 */

import { useEffect, useCallback, useRef } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { getPublicClient } from '@/lib/api-client'

type ResourceType = 'mcp' | 'agent'

/**
 * Records a page view when the component mounts.
 * Fires at most once per mount (StrictMode safe). On success, invalidates the
 * matching detail and list queries so the updated view_count surfaces in the
 * UI immediately instead of lagging behind a stale cache.
 */
export function useRecordView(type: ResourceType, namespace?: string, slug?: string) {
  const fired = useRef(false)
  const qc = useQueryClient()

  useEffect(() => {
    if (!namespace || !slug || fired.current) return
    fired.current = true
    const api = getPublicClient()
    // Branch on type so each POST call uses a literal path string. openapi-fetch
    // types each endpoint by its exact path literal — a ternary would widen to
    // `string` and force us back to `as any`.
    const req =
      type === 'mcp'
        ? api.POST('/api/v1/mcp/servers/{namespace}/{slug}/view', {
            params: { path: { namespace, slug } },
          })
        : api.POST('/api/v1/agents/{namespace}/{slug}/view', {
            params: { path: { namespace, slug } },
          })
    req
      .then(() => {
        // Bump the displayed count: refetch the detail query for this page and
        // invalidate any list query so returning to the grid shows the new value.
        const detailKey = type === 'mcp' ? ['mcp-server', namespace, slug] : ['agent', namespace, slug]
        const listKey = type === 'mcp' ? ['mcp-servers'] : ['agents']
        qc.invalidateQueries({ queryKey: detailKey })
        qc.invalidateQueries({ queryKey: listKey })
      })
      .catch(() => {})
  }, [type, namespace, slug, qc])
}

/**
 * Returns a callback that records a copy event.
 */
export function useRecordCopy(type: ResourceType, namespace?: string, slug?: string) {
  const qc = useQueryClient()
  return useCallback(() => {
    if (!namespace || !slug) return
    const api = getPublicClient()
    // Same literal-path branching as useRecordView — see the comment there.
    const req =
      type === 'mcp'
        ? api.POST('/api/v1/mcp/servers/{namespace}/{slug}/copy', {
            params: { path: { namespace, slug } },
          })
        : api.POST('/api/v1/agents/{namespace}/{slug}/copy', {
            params: { path: { namespace, slug } },
          })
    req
      .then(() => {
        const detailKey = type === 'mcp' ? ['mcp-server', namespace, slug] : ['agent', namespace, slug]
        qc.invalidateQueries({ queryKey: detailKey })
      })
      .catch(() => {})
  }, [type, namespace, slug, qc])
}
