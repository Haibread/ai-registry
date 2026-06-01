import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { Plus, UserCog, ArrowRight, ShieldCheck } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { useAuthClient } from '@/lib/api-client'
import { formatDate } from '@/lib/utils'

export default function AdminUserList() {
  const api = useAuthClient()

  const { data } = useQuery({
    queryKey: ['admin-users'],
    queryFn: () => api.GET('/api/v1/users', { params: { query: { limit: 100 } } }).then(r => r.data),
    enabled: true,
  })

  const users = data?.items ?? []

  return (
    <div className="space-y-4 max-w-4xl mx-auto">
      <div className="flex items-center justify-between gap-3 flex-wrap">
        <div>
          <h1 className="text-2xl font-bold">Users</h1>
          <p className="text-muted-foreground mt-1">
            {users.length} {users.length === 1 ? 'user' : 'users'}
          </p>
        </div>
        <Button asChild>
          <Link to="/admin/users/new" className="flex items-center gap-1.5">
            <Plus className="h-4 w-4" aria-hidden="true" /> New User
          </Link>
        </Button>
      </div>

      <p className="text-sm text-muted-foreground">
        Users are principals: federated (OIDC), local (email + password), both, or
        invited (no credential yet). Role grants attach to users or groups.
      </p>

      {users.length === 0 ? (
        <div className="flex flex-col items-center gap-3 py-16 text-center">
          <UserCog className="h-10 w-10 text-muted-foreground/40" aria-hidden="true" />
          <p className="text-muted-foreground font-medium">No users yet.</p>
          <Button asChild size="sm">
            <Link to="/admin/users/new">Create a user</Link>
          </Button>
        </div>
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Email</TableHead>
              <TableHead className="hidden md:table-cell">Display name</TableHead>
              <TableHead>Status</TableHead>
              <TableHead className="hidden lg:table-cell">Created</TableHead>
              <TableHead />
            </TableRow>
          </TableHeader>
          <TableBody>
            {users.map((u) => (
              <TableRow key={u.id}>
                <TableCell className="font-mono text-sm">{u.email}</TableCell>
                <TableCell className="font-medium hidden md:table-cell">{u.display_name || '—'}</TableCell>
                <TableCell className="space-x-1">
                  {u.is_server_admin && (
                    <Badge variant="success" className="gap-1">
                      <ShieldCheck className="h-3 w-3" aria-hidden="true" /> Server Admin
                    </Badge>
                  )}
                  {u.disabled && <Badge variant="muted">Disabled</Badge>}
                  {!u.is_server_admin && !u.disabled && <span className="text-muted-foreground text-sm">—</span>}
                </TableCell>
                <TableCell className="text-muted-foreground hidden lg:table-cell">{formatDate(u.created_at)}</TableCell>
                <TableCell className="text-right">
                  <Button variant="outline" size="sm" asChild>
                    <Link to={`/admin/users/${u.id}`}>
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
