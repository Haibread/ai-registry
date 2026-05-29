import { GrantsSection } from '@/components/admin/grants-section'

export default function AdminGrants() {
  return (
    <div className="space-y-4 max-w-3xl mx-auto">
      <div>
        <h1 className="text-2xl font-bold">Global grants</h1>
        <p className="text-muted-foreground mt-1">
          All-publishers role grants. These apply on every publisher and are
          reserved for Server Admins — prefer per-publisher grants on the
          publisher's own page.
        </p>
      </div>
      <GrantsSection />
    </div>
  )
}
