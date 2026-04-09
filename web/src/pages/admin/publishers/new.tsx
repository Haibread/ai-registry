import { useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { useMutation } from '@tanstack/react-query'
import { ArrowLeft, AlertCircle } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { getAuthClient } from '@/lib/api-client'
import { useAuth } from '@/auth/AuthContext'

export default function AdminPublisherNew() {
  const { accessToken } = useAuth()
  const navigate = useNavigate()
  const [errorMsg, setErrorMsg] = useState<string | null>(null)

  const mutation = useMutation({
    mutationFn: async (formData: FormData) => {
      const slug = (formData.get('slug') as string).trim()
      const name = (formData.get('name') as string).trim()

      if (!slug || !name) {
        throw new Error('Slug and name are required.')
      }

      const client = getAuthClient(accessToken!)
      const { error } = await client.POST('/api/v1/publishers', {
        body: {
          slug,
          name,
          contact: (formData.get('contact') as string) || undefined,
        },
      })

      if (error) {
        const msg = (error as { title?: string } | undefined)?.title
        throw new Error(msg ?? 'Failed to create publisher. The slug may already be in use.')
      }
    },
    onSuccess: () => {
      navigate('/admin/publishers')
    },
    onError: (err: Error) => {
      setErrorMsg(err.message)
    },
  })

  function handleSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault()
    setErrorMsg(null)
    mutation.mutate(new FormData(e.currentTarget))
  }

  return (
    <div className="space-y-6 max-w-lg mx-auto">
      <nav aria-label="Breadcrumb" className="flex items-center gap-1.5 text-sm text-muted-foreground">
        <Link to="/admin/publishers" className="flex items-center gap-1 hover:text-foreground transition-colors">
          <ArrowLeft className="h-3.5 w-3.5" aria-hidden="true" />
          Publishers
        </Link>
        <span aria-hidden="true">/</span>
        <span className="text-foreground">New Publisher</span>
      </nav>

      <h1 className="text-2xl font-bold">New Publisher</h1>

      {errorMsg && (
        <div role="alert" className="flex items-start gap-2 rounded-md border border-destructive/50 bg-destructive/10 px-4 py-3 text-sm text-destructive">
          <AlertCircle className="h-4 w-4 mt-0.5 shrink-0" aria-hidden="true" />
          <p>{errorMsg}</p>
        </div>
      )}

      <form onSubmit={handleSubmit}>
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Publisher Details</CardTitle>
            <CardDescription>Publishers are namespaces for MCP servers and agents.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-1.5">
              <Label htmlFor="slug">
                Slug <span className="text-destructive" aria-hidden="true">*</span>
              </Label>
              <Input
                id="slug"
                name="slug"
                placeholder="my-org"
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
              <Input id="name" name="name" placeholder="My Organization" required aria-required="true" />
            </div>

            <div className="space-y-1.5">
              <Label htmlFor="contact">Contact email</Label>
              <Input id="contact" name="contact" type="email" placeholder="team@example.com" />
            </div>
          </CardContent>
        </Card>

        <Button type="submit" className="w-full mt-6" disabled={mutation.isPending}>
          {mutation.isPending ? 'Creating…' : 'Create Publisher'}
        </Button>
      </form>
    </div>
  )
}
