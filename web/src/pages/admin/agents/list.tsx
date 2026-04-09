import { useQuery } from '@tanstack/react-query'
import { Link, useSearchParams } from 'react-router-dom'
import { Plus } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { StatusBadge, VisibilityBadge } from '@/components/ui/badge'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { FilterBar } from '@/components/ui/filter-bar'
import { getAuthClient } from '@/lib/api-client'
import { formatDate } from '@/lib/utils'
import { useAuth } from '@/auth/AuthContext'

const PAGE_LIMIT = 50

export default function AdminAgentList() {
  const { accessToken } = useAuth()
  const [searchParams] = useSearchParams()
  const q = searchParams.get('q') ?? undefined
  const namespace = searchParams.get('namespace') ?? undefined
  const status = searchParams.get('status') ?? undefined
  const visibility = searchParams.get('visibility') ?? undefined
  const cursor = searchParams.get('cursor') ?? undefined

  const api = getAuthClient(accessToken ?? '')
  const { data } = useQuery({
    queryKey: ['admin-agents', { q, namespace, status, visibility, cursor }],
    queryFn: () => api.GET('/api/v1/agents', {
      params: {
        query: {
          limit: PAGE_LIMIT,
          q,
          namespace,
          cursor,
          status: status as 'draft' | 'published' | 'deprecated' | undefined,
          visibility: visibility as 'public' | 'private' | undefined,
        },
      },
    }).then(r => r.data),
    enabled: !!accessToken,
  })

  const agents = data?.items ?? []

  const nextParams = new URLSearchParams()
  if (q) nextParams.set('q', q)
  if (namespace) nextParams.set('namespace', namespace)
  if (status) nextParams.set('status', status)
  if (visibility) nextParams.set('visibility', visibility)
  if (data?.next_cursor) nextParams.set('cursor', data.next_cursor)

  return (
    <div className="space-y-4 max-w-5xl mx-auto">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">Agents</h1>
          <p className="text-muted-foreground mt-1">
            {agents.length}{data?.total_count && data.total_count > agents.length ? ` of ${data.total_count}` : ''} entr{agents.length !== 1 ? 'ies' : 'y'}
          </p>
        </div>
        <Button asChild>
          <Link to="/admin/agents/new" className="flex items-center gap-1.5">
            <Plus className="h-4 w-4" /> New Agent
          </Link>
        </Button>
      </div>

      <FilterBar
        q={q}
        namespace={namespace}
        status={status}
        visibility={visibility}
        statusOptions={['draft', 'published', 'deprecated']}
        showVisibility
        searchPlaceholder="Search agents…"
      />

      {agents.length === 0 ? (
        <p className="text-muted-foreground py-8 text-center">
          {q || namespace || status || visibility
            ? 'No agents match your filters.'
            : 'No agents yet.'}
        </p>
      ) : (
        <>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Namespace / Slug</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Visibility</TableHead>
                <TableHead>Updated</TableHead>
                <TableHead />
              </TableRow>
            </TableHeader>
            <TableBody>
              {agents.map((a) => (
                <TableRow key={a.id}>
                  <TableCell className="font-medium">{a.name}</TableCell>
                  <TableCell className="font-mono text-sm text-muted-foreground">
                    {a.namespace}/{a.slug}
                  </TableCell>
                  <TableCell>
                    <StatusBadge status={a.status} />
                  </TableCell>
                  <TableCell>
                    <VisibilityBadge visibility={a.visibility} />
                  </TableCell>
                  <TableCell className="text-muted-foreground">{formatDate(a.updated_at)}</TableCell>
                  <TableCell>
                    <Button variant="ghost" size="sm" asChild>
                      <Link to={`/admin/agents/${a.namespace}/${a.slug}`}>Manage</Link>
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>

          {data?.next_cursor && (
            <div className="flex justify-center pt-2">
              <Button variant="outline" asChild>
                <Link to={`/admin/agents?${nextParams.toString()}`}>Load more</Link>
              </Button>
            </div>
          )}
        </>
      )}
    </div>
  )
}
