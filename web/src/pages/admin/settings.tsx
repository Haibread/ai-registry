import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Settings as SettingsIcon } from 'lucide-react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Skeleton } from '@/components/ui/skeleton'
import { EmptyState } from '@/components/ui/empty-state'
import { useAuthClient } from '@/lib/api-client'
import { useAuth } from '@/auth/AuthContext'
import { usePublisher } from '@/auth/PublisherContext'
import { usePermissions } from '@/auth/useMe'

// AdminSettings edits the currently selected publisher's metadata (name +
// contact). Scoped via the publisher switcher like the rest of the admin
// shell, and reachable only by a publisher Admin (or Server Admin) — the nav
// hides it otherwise; this guard also covers a deep link. Deleting a publisher
// stays a Server-Admin action on the global Publishers page.
export default function AdminSettings() {
  const { currentSlug, current } = usePublisher()
  const perms = usePermissions()
  const { accessToken } = useAuth()
  const api = useAuthClient()
  const queryClient = useQueryClient()

  const slug = currentSlug ?? ''

  const { data: publisher, isPending } = useQuery({
    queryKey: ['admin-publisher', slug],
    queryFn: () =>
      api.GET('/api/v1/publishers/{slug}', { params: { path: { slug } } }).then((r) => r.data),
    enabled: !!accessToken && !!slug,
  })

  const save = useMutation({
    mutationFn: async (body: { name: string; contact: string }) => {
      const { error } = await api.PATCH('/api/v1/publishers/{slug}', {
        params: { path: { slug } },
        body,
      })
      if (error) {
        const e = error as { detail?: string; title?: string }
        throw new Error(e?.detail ?? e?.title ?? 'Update failed.')
      }
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin-publisher', slug] })
      queryClient.invalidateQueries({ queryKey: ['publisher-stats', slug] })
      queryClient.invalidateQueries({ queryKey: ['admin-publishers'] })
      toast.success('Publisher updated')
    },
    onError: (e: Error) => toast.error(e.message),
  })

  if (!currentSlug) {
    return (
      <div className="max-w-3xl mx-auto">
        <EmptyState
          icon={<SettingsIcon className="h-10 w-10" />}
          title="Select a publisher"
          description="Pick a publisher from the switcher to edit its settings."
        />
      </div>
    )
  }

  if (!perms.canAdmin(currentSlug)) {
    return (
      <div className="max-w-3xl mx-auto">
        <EmptyState
          icon={<SettingsIcon className="h-10 w-10" />}
          title="Admin access required"
          description={`You need the Admin role on ${current?.name ?? currentSlug} to edit its settings.`}
        />
      </div>
    )
  }

  return (
    <div className="space-y-6 max-w-xl mx-auto">
      <div>
        <h1 className="text-2xl font-bold">Settings</h1>
        <p className="text-muted-foreground mt-1 text-sm">
          Edit the name and contact for <span className="font-mono">{currentSlug}</span>. The slug
          is permanent.
        </p>
      </div>

      {isPending ? (
        <div className="space-y-4">
          <Skeleton className="h-10 rounded-md" />
          <Skeleton className="h-10 rounded-md" />
          <Skeleton className="h-9 w-32 rounded-md" />
        </div>
      ) : !publisher ? (
        <EmptyState
          icon={<SettingsIcon className="h-10 w-10" />}
          title="Publisher unavailable"
          description="This publisher could not be loaded. Try reselecting it from the switcher."
        />
      ) : (
        <form
          className="space-y-4"
          onSubmit={(e) => {
            e.preventDefault()
            const fd = new FormData(e.currentTarget)
            save.mutate({
              name: (fd.get('name') as string).trim(),
              contact: (fd.get('contact') as string).trim(),
            })
          }}
        >
          <div className="space-y-1.5">
            <Label htmlFor="slug">Slug</Label>
            <Input id="slug" value={publisher.slug} readOnly disabled className="font-mono" />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="name">
              Name <span className="text-destructive">*</span>
            </Label>
            <Input id="name" name="name" defaultValue={publisher.name} required />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="contact">Contact</Label>
            <Input
              id="contact"
              name="contact"
              defaultValue={publisher.contact ?? ''}
              placeholder="ops@example.com"
            />
          </div>
          <Button type="submit" disabled={save.isPending}>
            {save.isPending ? 'Saving…' : 'Save changes'}
          </Button>
        </form>
      )}
    </div>
  )
}
