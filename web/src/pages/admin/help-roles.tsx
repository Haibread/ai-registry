/**
 * Role matrix — a short reference for what each role can actually do
 * (UI/UX review P3: the grants/members surfaces used the role names without
 * ever defining them). Linked from the Members page and the grants editor.
 */

import { Link } from 'react-router-dom'
import { ArrowLeft, Shield } from 'lucide-react'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'

interface RoleRow {
  role: string
  scope: string
  can: string
}

const ROLE_MATRIX: RoleRow[] = [
  {
    role: 'Viewer',
    scope: 'Publisher',
    can: 'Read the publisher’s private entries and drafts. No writes.',
  },
  {
    role: 'Editor',
    scope: 'Publisher',
    can: 'Everything Viewer can, plus create entries, author versions, submit for review, request deletion, and propose metadata/visibility/lifecycle changes (each goes through the review queue).',
  },
  {
    role: 'Reviewer',
    scope: 'Publisher or global',
    can: 'Everything Editor can, plus approve or reject submissions on the review queue and publish versions directly.',
  },
  {
    role: 'Admin',
    scope: 'Publisher',
    can: 'Everything Reviewer can on the publisher, plus manage its members, grants, and settings.',
  },
  {
    role: 'Server Admin',
    scope: 'Whole registry',
    can: 'Everything, everywhere, applied immediately (bypasses the review queue). Manages users, groups, publishers, global grants, reports, and the audit log.',
  },
]

export default function AdminHelpRoles() {
  return (
    <div className="space-y-6 max-w-3xl mx-auto">
      <nav aria-label="Breadcrumb" className="flex items-center gap-1.5 text-sm text-muted-foreground">
        <Link to="/admin" className="flex items-center gap-1 hover:text-foreground transition-colors">
          <ArrowLeft className="h-3.5 w-3.5" aria-hidden="true" />
          Dashboard
        </Link>
        <span aria-hidden="true">/</span>
        <span className="text-foreground">Roles</span>
      </nav>

      <div className="flex items-center gap-3">
        <Shield className="h-6 w-6 text-muted-foreground" aria-hidden="true" />
        <h1 className="text-2xl font-bold">Roles</h1>
      </div>

      <p className="text-sm text-muted-foreground max-w-prose">
        Roles are granted per publisher (or globally) to users or groups, and
        each role includes everything below it. Group grants bind an identity
        provider claim to a role and are managed by Server Admins.
      </p>

      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Role</TableHead>
            <TableHead>Scope</TableHead>
            <TableHead>What it can do</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {ROLE_MATRIX.map((r) => (
            <TableRow key={r.role}>
              <TableCell className="font-medium whitespace-nowrap align-top">{r.role}</TableCell>
              <TableCell className="text-muted-foreground whitespace-nowrap align-top">{r.scope}</TableCell>
              <TableCell className="text-sm max-w-prose">{r.can}</TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>

      <p className="text-sm text-muted-foreground max-w-prose">
        Grants labelled <span className="font-mono">config</span> are seeded
        from the server&apos;s bootstrap file on every start — remove them
        there, not here.
      </p>
    </div>
  )
}
