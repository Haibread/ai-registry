/**
 * useRecordEvent — fire-and-forget view/copy tracking hooks.
 *
 * The POST requests are non-blocking and errors are silently swallowed.
 */

import { useEffect, useCallback, useRef } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { getPublicClient } from '@/lib/api-client'

/**
 * Records a page view when the component mounts.
 * Fires at most once per mount (StrictMode safe). On success, invalidates the
 * matching detail and list queries so the updated view_count surfaces in the
 * UI immediately instead of lagging behind a stale cache.
 */
export function useRecordView(namespace?: string, slug?: string) {
  const fired = useRef(false)
  const qc = useQueryClient()

  useEffect(() => {
    if (!namespace || !slug || fired.current) return
    fired.current = true
    getPublicClient()
      .POST('/api/v1/mcp/servers/{namespace}/{slug}/view', {
        params: { path: { namespace, slug } },
      })
      .then(() => {
        // Bump the displayed count: refetch the detail query for this page and
        // invalidate any list query so returning to the grid shows the new value.
        qc.invalidateQueries({ queryKey: ['mcp-server', namespace, slug] })
        qc.invalidateQueries({ queryKey: ['mcp-servers'] })
      })
      .catch(() => {})
  }, [namespace, slug, qc])
}

/**
 * Returns a callback that records a copy event.
 */
export function useRecordCopy(namespace?: string, slug?: string) {
  const qc = useQueryClient()
  return useCallback(() => {
    if (!namespace || !slug) return
    getPublicClient()
      .POST('/api/v1/mcp/servers/{namespace}/{slug}/copy', {
        params: { path: { namespace, slug } },
      })
      .then(() => {
        qc.invalidateQueries({ queryKey: ['mcp-server', namespace, slug] })
      })
      .catch(() => {})
  }, [namespace, slug, qc])
}
