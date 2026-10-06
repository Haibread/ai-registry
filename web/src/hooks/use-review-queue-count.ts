import { useQuery } from '@tanstack/react-query'
import { useAuthClient } from '@/lib/api-client'

export interface ReviewQueueCount {
  count: number
  /** More items are waiting than the page fetched; display "99+". */
  hasMore: boolean
}

/** Pending review-queue items for the signed-in reviewer. Polls every 30s so
 *  a reviewer who leaves the admin open notices new submissions. */
export function useReviewQueueCount(): ReviewQueueCount | undefined {
  const api = useAuthClient()
  const { data } = useQuery({
    queryKey: ['admin-review-queue-count'],
    queryFn: async () => {
      // Limit 99 + next_cursor is enough to render "99+"; a larger page would
      // cost bandwidth on every admin page load for no gain.
      const r = await api.GET('/api/v1/review-queue', {
        params: { query: { limit: 99 } },
      })
      const items = r.data?.items ?? []
      return { count: items.length, hasMore: !!r.data?.next_cursor }
    },
    refetchInterval: 30_000,
  })
  return data
}

export function formatQueueCount({ count, hasMore }: ReviewQueueCount): string {
  return hasMore || count >= 100 ? '99+' : String(count)
}
