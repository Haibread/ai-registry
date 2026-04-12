/**
 * useRecordEvent — fire-and-forget view/copy tracking hooks.
 *
 * The POST requests are non-blocking and errors are silently swallowed.
 */

import { useEffect, useCallback, useRef } from 'react'
import { getPublicClient } from '@/lib/api-client'

type ResourceType = 'mcp' | 'agent'

/**
 * Records a page view when the component mounts.
 * Fires at most once per mount (StrictMode safe).
 */
export function useRecordView(type: ResourceType, namespace?: string, slug?: string) {
  const fired = useRef(false)

  useEffect(() => {
    if (!namespace || !slug || fired.current) return
    fired.current = true
    const api = getPublicClient()
    const path =
      type === 'mcp'
        ? '/api/v1/mcp/servers/{namespace}/{slug}/view'
        : '/api/v1/agents/{namespace}/{slug}/view'
    api.POST(path as any, { params: { path: { namespace, slug } } }).catch(() => {})
  }, [type, namespace, slug])
}

/**
 * Returns a callback that records a copy event.
 */
export function useRecordCopy(type: ResourceType, namespace?: string, slug?: string) {
  return useCallback(() => {
    if (!namespace || !slug) return
    const api = getPublicClient()
    const path =
      type === 'mcp'
        ? '/api/v1/mcp/servers/{namespace}/{slug}/copy'
        : '/api/v1/agents/{namespace}/{slug}/copy'
    api.POST(path as any, { params: { path: { namespace, slug } } }).catch(() => {})
  }, [type, namespace, slug])
}
