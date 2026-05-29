import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { Plus, UsersRound, ArrowRight } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { useAuthClient } from '@/lib/api-client'
import { formatDate } from '@/lib/utils'
import { useAuth } from '@/auth/AuthContext'

export default function AdminGroupList() {
  const { accessToken } = useAuth()
  const api = useAuthClient()

  const { data } = useQuery({
    queryKey: ['admin-groups'],
    queryFn: () => api.GET('/api/v1/groups', { params: { query: { limit: 100 } } }).then(r => r.data),
    enabled: !!accessToken,
  })

  const groups = data?.items ?? []

  return (
    <div className="space-y-4 max-w-4xl mx-auto">
      <div className="flex items-center justify-between gap-3 flex-wrap">
        <div>
          <h1 className="text-2xl font-bold">Groups</h1>
          <p className="text-muted-foreground mt-1">
            {groups.length} {groups.length === 1 ? 'group' : 'groups'}
          </p>
        </div>
        <Button asChild>
          <Link to="/admin/groups/new" className="flex items-center gap-1.5">
            <Plus className="h-4 w-4" aria-hidden="true" /> New Group
          </Link>
        </Button>
      </div>

      <p className="text-sm text-muted-foreground">
        Groups (teams) receive role grants on publishers. A group's slug is also
        matched against the <code>groups</code> claim in OIDC tokens, so a
        federated user named in the claim inherits the group's grants.
      </p>

      {groups.length === 0 ? (
        <div className="flex flex-col items-center gap-3 py-16 text-center">
          <UsersRound className="h-10 w-10 text-muted-foreground/40" aria-hidden="true" />
          <p className="text-muted-foreground font-medium">No groups yet.</p>
          <Button asChild size="sm">
            <Link to="/admin/groups/new">Create your first group</Link>
          </Button>
        </div>
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Slug</TableHead>
              <TableHead>Name</TableHead>
              <TableHead className="hidden md:table-cell">Description</TableHead>
              <TableHead className="hidden lg:table-cell">Created</TableHead>
              <TableHead />
            </TableRow>
          </TableHeader>
          <TableBody>
            {groups.map((g) => (
              <TableRow key={g.id}>
                <TableCell className="font-mono text-sm">{g.slug}</TableCell>
                <TableCell className="font-medium">{g.name}</TableCell>
                <TableCell className="text-muted-foreground hidden md:table-cell">{g.description || '—'}</TableCell>
                <TableCell className="text-muted-foreground hidden lg:table-cell">{formatDate(g.created_at)}</TableCell>
                <TableCell className="text-right">
                  <Button variant="outline" size="sm" asChild>
                    <Link to={`/admin/groups/${g.slug}`}>
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
