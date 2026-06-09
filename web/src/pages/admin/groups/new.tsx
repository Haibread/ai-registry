import { useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { useMutation } from '@tanstack/react-query'
import { toast } from 'sonner'
import { ArrowLeft, AlertCircle } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { useAuthClient } from '@/lib/api-client'

export default function AdminGroupNew() {
  const api = useAuthClient()
  const navigate = useNavigate()
  const [errorMsg, setErrorMsg] = useState<string | null>(null)

  const mutation = useMutation({
    mutationFn: async (formData: FormData) => {
      const slug = (formData.get('slug') as string).trim()
      const name = (formData.get('name') as string).trim()
      if (!slug || !name) {
        throw new Error('Slug and name are required.')
      }
      const { error } = await api.POST('/api/v1/groups', {
        body: {
          slug,
          name,
          description: (formData.get('description') as string) || undefined,
        },
      })
      if (error) {
        const msg = (error as { title?: string } | undefined)?.title
        throw new Error(msg ?? 'Failed to create group. The slug may already be in use.')
      }
    },
    onSuccess: () => { toast.success('Group created'); navigate('/admin/groups') },
    onError: (err: Error) => setErrorMsg(err.message),
  })

  function handleSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault()
    setErrorMsg(null)
    mutation.mutate(new FormData(e.currentTarget))
  }

  return (
    <div className="space-y-6 max-w-lg mx-auto">
      <nav aria-label="Breadcrumb" className="flex items-center gap-1.5 text-sm text-muted-foreground">
        <Link to="/admin/groups" className="flex items-center gap-1 hover:text-foreground transition-colors">
          <ArrowLeft className="h-3.5 w-3.5" aria-hidden="true" />
          Groups
        </Link>
        <span aria-hidden="true">/</span>
        <span className="text-foreground">New Group</span>
      </nav>

      <h1 className="text-2xl font-bold">New Group</h1>

      {errorMsg && (
        <div role="alert" className="flex items-start gap-2 rounded-md border border-destructive/50 bg-destructive/10 px-4 py-3 text-sm text-destructive">
          <AlertCircle className="h-4 w-4 mt-0.5 shrink-0" aria-hidden="true" />
          <p>{errorMsg}</p>
        </div>
      )}

      <form onSubmit={handleSubmit}>
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Group Details</CardTitle>
            <CardDescription>
              The slug is matched against the OIDC <code>groups</code> claim, so set it
              to the IdP group name when federating.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-1.5">
              <Label htmlFor="slug">
                Slug <span className="text-destructive" aria-hidden="true">*</span>
              </Label>
              <Input
                id="slug"
                name="slug"
                placeholder="platform-team"
                pattern="^[a-z0-9-]+"
                title="Lowercase letters, numbers, and hyphens only"
                required
                aria-required="true"
              />
              <p className="text-xs text-muted-foreground">Lowercase letters, numbers, and hyphens only.</p>
            </div>

            <div className="space-y-1.5">
              <Label htmlFor="name">
                Name <span className="text-destructive" aria-hidden="true">*</span>
              </Label>
              <Input id="name" name="name" placeholder="Platform Team" required aria-required="true" />
            </div>

            <div className="space-y-1.5">
              <Label htmlFor="description">Description</Label>
              <Input id="description" name="description" placeholder="Owns shared infrastructure" />
            </div>
          </CardContent>
        </Card>

        <Button type="submit" className="w-full mt-6" disabled={mutation.isPending}>
          {mutation.isPending ? 'Creating…' : 'Create Group'}
        </Button>
      </form>
    </div>
  )
}
