import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { Plus, CheckCircle2, Circle, Building2, ArrowRight } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { TableSkeleton } from '@/components/ui/table-skeleton'
import { EmptyState } from '@/components/ui/empty-state'
import { ErrorState } from '@/components/ui/error-state'
import { useAuthClient } from '@/lib/api-client'
import { formatDate, problemMessage } from '@/lib/utils'

export default function AdminPublisherList() {
  const api = useAuthClient()

  const { data, isPending, isError } = useQuery({
    queryKey: ['admin-publishers'],
    queryFn: async () => {
      const { data, error } = await api.GET('/api/v1/publishers', { params: { query: { limit: 100 } } })
      if (error) throw new Error(problemMessage(error, 'Failed to load publishers.'))
      return data
    },
    enabled: true,
  })

  const publishers = data?.items ?? []

  return (
    <div className="space-y-4 max-w-4xl mx-auto">
      <div className="flex items-center justify-between gap-3 flex-wrap">
        <div>
          <h1 className="text-2xl font-bold">Publishers</h1>
          <p className="text-muted-foreground mt-1">
            {publishers.length} {publishers.length === 1 ? 'publisher' : 'publishers'}
          </p>
        </div>
        <Button asChild>
          <Link to="/admin/publishers/new" className="flex items-center gap-1.5">
            <Plus className="h-4 w-4" aria-hidden="true" /> New Publisher
          </Link>
        </Button>
      </div>

      {isPending ? (
        <TableSkeleton rows={6} cols={6} />
      ) : isError ? (
        <ErrorState message="Failed to load publishers." />
      ) : publishers.length === 0 ? (
        <EmptyState
          icon={<Building2 className="h-10 w-10" aria-hidden="true" />}
          title="No publishers yet."
          description="Publishers own MCP servers and agents and define their namespace."
          action={
            <Button asChild size="sm">
              <Link to="/admin/publishers/new">Create your first publisher</Link>
            </Button>
          }
        />
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Slug</TableHead>
              <TableHead>Name</TableHead>
              <TableHead className="hidden md:table-cell">Contact</TableHead>
              <TableHead>Verified</TableHead>
              <TableHead className="hidden lg:table-cell">Created</TableHead>
              <TableHead />
            </TableRow>
          </TableHeader>
          <TableBody>
            {publishers.map((p) => (
              <TableRow key={p.id}>
                <TableCell className="font-mono text-sm">{p.slug}</TableCell>
                <TableCell className="font-medium">{p.name}</TableCell>
                <TableCell className="text-muted-foreground hidden md:table-cell">{p.contact ?? '—'}</TableCell>
                <TableCell>
                  {p.verified ? (
                    <CheckCircle2 className="h-4 w-4 text-green-600" aria-label="Verified" />
                  ) : (
                    <Circle className="h-4 w-4 text-muted-foreground" aria-label="Unverified" />
                  )}
                </TableCell>
                <TableCell className="text-muted-foreground hidden lg:table-cell">{formatDate(p.created_at)}</TableCell>
                <TableCell className="text-right">
                  <Button variant="outline" size="sm" asChild>
                    <Link to={`/admin/publishers/${p.slug}`}>
                      Manage
                      <ArrowRight className="h-3.5 w-3.5" aria-hidden="true" />
                    </Link>
                  </Button>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}
    </div>
  )
}
