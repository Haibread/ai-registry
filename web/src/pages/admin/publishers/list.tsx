import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { Plus, CheckCircle2, Circle, Building2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { useAuthClient } from '@/lib/api-client'
import { formatDate } from '@/lib/utils'
import { useAuth } from '@/auth/AuthContext'

export default function AdminPublisherList() {
  const { accessToken } = useAuth()
  const api = useAuthClient()

  const { data } = useQuery({
    queryKey: ['admin-publishers'],
    queryFn: () => api.GET('/api/v1/publishers', { params: { query: { limit: 100 } } }).then(r => r.data),
    enabled: !!accessToken,
  })

  const publishers = data?.items ?? []

  return (
    <div className="space-y-4 max-w-4xl mx-auto">
      <div className="flex items-center justify-between">
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

      {publishers.length === 0 ? (
        <div className="flex flex-col items-center gap-3 py-16 text-center">
          <Building2 className="h-10 w-10 text-muted-foreground/40" aria-hidden="true" />
          <p className="text-muted-foreground font-medium">No publishers yet.</p>
          <p className="text-sm text-muted-foreground">Publishers are namespaces for MCP servers and agents.</p>
          <Button asChild size="sm">
            <Link to="/admin/publishers/new">Create your first publisher</Link>
          </Button>
        </div>
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Slug</TableHead>
              <TableHead>Name</TableHead>
              <TableHead>Contact</TableHead>
              <TableHead>Verified</TableHead>
              <TableHead>Created</TableHead>
              <TableHead />
            </TableRow>
          </TableHeader>
          <TableBody>
            {publishers.map((p) => (
              <TableRow key={p.id}>
                <TableCell className="font-mono text-sm">{p.slug}</TableCell>
                <TableCell className="font-medium">{p.name}</TableCell>
                <TableCell className="text-muted-foreground">{p.contact ?? '—'}</TableCell>
                <TableCell>
                  {p.verified ? (
                    <CheckCircle2 className="h-4 w-4 text-green-600" aria-label="Verified" />
                  ) : (
                    <Circle className="h-4 w-4 text-muted-foreground" aria-label="Unverified" />
                  )}
                </TableCell>
                <TableCell className="text-muted-foreground">{formatDate(p.created_at)}</TableCell>
                <TableCell>
                  <Button variant="ghost" size="sm" asChild>
                    <Link to={`/admin/publishers/${p.slug}`}>Manage</Link>
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
