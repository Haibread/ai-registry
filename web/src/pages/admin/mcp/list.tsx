import { useQuery } from '@tanstack/react-query'
import { Link, useSearchParams } from 'react-router-dom'
import { Plus } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { StatusBadge, VisibilityBadge } from '@/components/ui/badge'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { FilterBar } from '@/components/ui/filter-bar'
import { useAuthClient } from '@/lib/api-client'
import { formatDate } from '@/lib/utils'
import { useAuth } from '@/auth/AuthContext'

const PAGE_LIMIT = 50

export default function AdminMCPList() {
  const { accessToken } = useAuth()
  const api = useAuthClient()
  const [searchParams] = useSearchParams()
  const q = searchParams.get('q') ?? undefined
  const namespace = searchParams.get('namespace') ?? undefined
  const status = searchParams.get('status') ?? undefined
  const visibility = searchParams.get('visibility') ?? undefined
  const cursor = searchParams.get('cursor') ?? undefined
  const { data } = useQuery({
    queryKey: ['admin-mcp', { q, namespace, status, visibility, cursor }],
    queryFn: () => api.GET('/api/v1/mcp/servers', {
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

  const servers = data?.items ?? []

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
          <h1 className="text-2xl font-bold">MCP Servers</h1>
          <p className="text-muted-foreground mt-1">
            {servers.length}{data?.total_count && data.total_count > servers.length ? ` of ${data.total_count}` : ''} entr{servers.length !== 1 ? 'ies' : 'y'}
          </p>
        </div>
        <Button asChild>
          <Link to="/admin/mcp/new" className="flex items-center gap-1.5">
            <Plus className="h-4 w-4" /> New Server
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
        searchPlaceholder="Search servers…"
      />

      {servers.length === 0 ? (
        <p className="text-muted-foreground py-8 text-center">
          {q || namespace || status || visibility
            ? 'No servers match your filters.'
            : 'No MCP servers yet.'}
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
              {servers.map((s) => (
                <TableRow key={s.id}>
                  <TableCell className="font-medium">{s.name}</TableCell>
                  <TableCell className="font-mono text-sm text-muted-foreground">
                    {s.namespace}/{s.slug}
                  </TableCell>
                  <TableCell>
                    <StatusBadge status={s.status} />
                  </TableCell>
                  <TableCell>
                    <VisibilityBadge visibility={s.visibility} />
                  </TableCell>
                  <TableCell className="text-muted-foreground">{formatDate(s.updated_at)}</TableCell>
                  <TableCell>
                    <Button variant="ghost" size="sm" asChild>
                      <Link to={`/admin/mcp/${s.namespace}/${s.slug}`}>Manage</Link>
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>

          {data?.next_cursor && (
            <div className="flex justify-center pt-2">
              <Button variant="outline" asChild>
                <Link to={`/admin/mcp?${nextParams.toString()}`}>Load more</Link>
              </Button>
            </div>
          )}
        </>
      )}
    </div>
  )
}
