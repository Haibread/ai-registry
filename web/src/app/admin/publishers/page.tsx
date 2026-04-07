import type { Metadata } from "next"
import Link from "next/link"
import { Plus, CheckCircle2, Circle } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { getApiClient } from "@/lib/api-client"
import { formatDate } from "@/lib/utils"

export const metadata: Metadata = { title: "Publishers" }

export default async function AdminPublishersPage() {
  const api = await getApiClient()
  const { data } = await api.GET("/api/v1/publishers", { params: { query: { limit: 100 } } })
  const publishers = data?.items ?? []

  return (
    <div className="space-y-4 max-w-4xl">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">Publishers</h1>
          <p className="text-muted-foreground mt-1">{publishers.length} total</p>
        </div>
        <Button asChild>
          <Link href="/admin/publishers/new" className="flex items-center gap-1.5">
            <Plus className="h-4 w-4" /> New Publisher
          </Link>
        </Button>
      </div>

      {publishers.length === 0 ? (
        <p className="text-muted-foreground py-8 text-center">No publishers yet.</p>
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Slug</TableHead>
              <TableHead>Name</TableHead>
              <TableHead>Contact</TableHead>
              <TableHead>Verified</TableHead>
              <TableHead>Created</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {publishers.map((p) => (
              <TableRow key={p.id}>
                <TableCell className="font-mono text-sm">{p.slug}</TableCell>
                <TableCell className="font-medium">{p.name}</TableCell>
                <TableCell className="text-muted-foreground">{p.contact ?? "—"}</TableCell>
                <TableCell>
                  {p.verified ? (
                    <CheckCircle2 className="h-4 w-4 text-green-600" />
                  ) : (
                    <Circle className="h-4 w-4 text-muted-foreground" />
                  )}
                </TableCell>
                <TableCell className="text-muted-foreground">{formatDate(p.created_at)}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}
    </div>
  )
}
