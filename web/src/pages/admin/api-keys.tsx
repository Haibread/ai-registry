import { Key } from 'lucide-react'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'

export default function AdminApiKeys() {
  return (
    <div className="space-y-4 max-w-xl mx-auto">
      <h1 className="text-2xl font-bold">API Keys</h1>

      <Card>
        <CardHeader>
          <div className="flex items-center gap-2">
            <Key className="h-5 w-5 text-muted-foreground" />
            <CardTitle className="text-base">Coming soon</CardTitle>
          </div>
          <CardDescription>
            Hashed API keys for machine-to-machine admin operations (CI/CD publish pipelines) are
            planned for Phase 5. Keys will be scoped per publisher and checked via{' '}
            <code className="text-xs">Authorization: Bearer apikey_…</code>.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <p className="text-sm text-muted-foreground">
            In the meantime, use your Keycloak access token for automated operations.
          </p>
        </CardContent>
      </Card>
    </div>
  )
}
