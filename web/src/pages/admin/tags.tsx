/**
 * Instance tags — Server-Admin curation of the instance-wide tag vocabulary.
 *
 * Publishers tick these tags when they create a version; the ticked slugs
 * freeze with the published version. Hence the lifecycle surfaced here:
 * a tag carried by any version can only be deactivated (hidden from new
 * publishes), while hard delete works for never-used tags (the API answers
 * 409 otherwise).
 */

import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { Plus, Tags } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { NativeSelect } from '@/components/ui/native-select'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { TableSkeleton } from '@/components/ui/table-skeleton'
import { EmptyState } from '@/components/ui/empty-state'
import { ErrorState } from '@/components/ui/error-state'
import { ConfirmDialog } from '@/components/ui/confirm-dialog'
import { TagBadge } from '@/components/ui/tag-badge'
import { useAuthClient } from '@/lib/api-client'
import { problemMessage } from '@/lib/utils'
import type { components } from '@/lib/schema'

type InstanceTag = components['schemas']['InstanceTag']
type TagColor = NonNullable<components['schemas']['CreateInstanceTagRequest']['color']>

const TAG_COLORS: TagColor[] = [
  'gray', 'red', 'orange', 'yellow', 'green', 'teal', 'blue', 'indigo', 'purple', 'pink',
]

interface TagFormState {
  slug: string
  name: string
  description: string
  color: TagColor
}

const emptyForm: TagFormState = { slug: '', name: '', description: '', color: 'gray' }

export default function AdminTagList() {
  const api = useAuthClient()
  const queryClient = useQueryClient()

  const [form, setForm] = useState<TagFormState>(emptyForm)
  const [editing, setEditing] = useState<InstanceTag | null>(null)
  const [deleting, setDeleting] = useState<InstanceTag | null>(null)

  const { data, isPending, isError } = useQuery({
    queryKey: ['instance-tags'],
    queryFn: async () => {
      // The operation declares only a 200, so openapi-fetch types `error` as
      // never — a missing body is the only failure signal to branch on.
      const r = await api.GET('/api/v1/tags')
      if (!r.data) throw new Error('Failed to load tags.')
      return r.data
    },
  })
  const tags = data?.items ?? []

  const invalidate = () => queryClient.invalidateQueries({ queryKey: ['instance-tags'] })

  const createMutation = useMutation({
    mutationFn: async (f: TagFormState) => {
      const r = await api.POST('/api/v1/tags', {
        body: {
          slug: f.slug,
          name: f.name,
          description: f.description || undefined,
          color: f.color,
        },
      })
      if (r.error) throw new Error(problemMessage(r.error, 'Failed to create tag.'))
      return r.data
    },
    onSuccess: (t) => {
      toast.success(`Tag "${t.name}" created`)
      setForm(emptyForm)
      invalidate()
    },
    onError: (e: Error) => toast.error(e.message),
  })

  const updateMutation = useMutation({
    mutationFn: async (p: { slug: string; body: components['schemas']['UpdateInstanceTagRequest'] }) => {
      const r = await api.PATCH('/api/v1/tags/{slug}', {
        params: { path: { slug: p.slug } },
        body: p.body,
      })
      if (r.error) throw new Error(problemMessage(r.error, 'Failed to update tag.'))
      return r.data
    },
    onSuccess: (t) => {
      toast.success(`Tag "${t.name}" updated`)
      setEditing(null)
      invalidate()
    },
    onError: (e: Error) => toast.error(e.message),
  })

  const deleteMutation = useMutation({
    mutationFn: async (slug: string) => {
      const r = await api.DELETE('/api/v1/tags/{slug}', { params: { path: { slug } } })
      if (r.error) throw new Error(problemMessage(r.error, 'Failed to delete tag.'))
    },
    onSuccess: () => {
      toast.success('Tag deleted')
      setDeleting(null)
      invalidate()
    },
    onError: (e: Error) => {
      // Most common cause: the tag is carried by a published (immutable)
      // version — the API refuses with 409 and the fix is to deactivate.
      toast.error(e.message)
      setDeleting(null)
    },
  })

  return (
    <div className="space-y-6 max-w-4xl mx-auto">
      <div>
        <h1 className="text-2xl font-bold">Instance tags</h1>
        <p className="text-muted-foreground mt-1">
          {tags.length} {tags.length === 1 ? 'tag' : 'tags'}
        </p>
      </div>

      <p className="text-sm text-muted-foreground max-w-prose">
        The instance-wide vocabulary publishers can tick when they publish a
        version (e.g. pricing or maturity markers). Ticked tags freeze with
        the published version, so a tag that is in use can only be
        deactivated — it stays visible on existing versions but can no longer
        be ticked on new ones.
      </p>

      {/* Create form */}
      <form
        className="grid gap-3 rounded-lg border p-4 sm:grid-cols-2 lg:grid-cols-5"
        onSubmit={(e) => {
          e.preventDefault()
          createMutation.mutate(form)
        }}
      >
        <div className="space-y-1.5">
          <Label htmlFor="tag-slug">Slug</Label>
          <Input
            id="tag-slug"
            name="slug"
            value={form.slug}
            onChange={(e) => setForm({ ...form, slug: e.target.value })}
            placeholder="early-access"
            pattern="[a-z0-9-]+"
            title="Lowercase letters, digits, and hyphens"
            required
          />
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="tag-name">Display name</Label>
          <Input
            id="tag-name"
            name="name"
            value={form.name}
            onChange={(e) => setForm({ ...form, name: e.target.value })}
            placeholder="Early Access"
            required
          />
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="tag-description">Description</Label>
          <Input
            id="tag-description"
            name="description"
            value={form.description}
            onChange={(e) => setForm({ ...form, description: e.target.value })}
            placeholder="Shown as a tooltip"
          />
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="tag-color">Color</Label>
          <NativeSelect
            id="tag-color"
            name="color"
            className="w-full"
            value={form.color}
            onChange={(e) => setForm({ ...form, color: e.target.value as TagColor })}
          >
            {TAG_COLORS.map((c) => (
              <option key={c} value={c}>
                {c}
              </option>
            ))}
          </NativeSelect>
        </div>
        <div className="flex items-end">
          <Button type="submit" disabled={createMutation.isPending} className="w-full lg:w-auto">
            <Plus className="h-4 w-4" aria-hidden="true" /> Add tag
          </Button>
        </div>
      </form>

      {isPending ? (
        <TableSkeleton rows={4} cols={5} />
      ) : isError ? (
        <ErrorState message="Failed to load tags." />
      ) : tags.length === 0 ? (
        <EmptyState
          icon={<Tags className="h-10 w-10" aria-hidden="true" />}
          title="No tags yet."
          description="Define the first tag above — publishers will see it as a checkbox on the publish form."
        />
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Tag</TableHead>
              <TableHead className="hidden md:table-cell">Slug</TableHead>
              <TableHead className="hidden lg:table-cell">Description</TableHead>
              <TableHead>Status</TableHead>
              <TableHead />
            </TableRow>
          </TableHeader>
          <TableBody>
            {tags.map((t) => (
              <TableRow key={t.slug} className={t.active ? undefined : 'opacity-60'}>
                <TableCell>
                  <TagBadge slug={t.slug} tag={t} />
                </TableCell>
                <TableCell className="font-mono text-sm hidden md:table-cell">{t.slug}</TableCell>
                <TableCell className="text-muted-foreground hidden lg:table-cell max-w-xs truncate">
                  {t.description || '—'}
                </TableCell>
                <TableCell className="space-x-1 whitespace-nowrap">
                  <span>{t.active ? 'Active' : 'Deactivated'}</span>
                  {t.managed && (
                    <Badge
                      variant="muted"
                      title="Reconciled from the server configuration (INSTANCE_TAGS / Helm api.instanceTags) — read-only here"
                    >
                      Managed
                    </Badge>
                  )}
                </TableCell>
                <TableCell className="text-right space-x-2 whitespace-nowrap">
                  {/* Managed tags are owned by the server configuration; the
                      API would answer 409, so the actions are disabled with
                      a pointer to where the change belongs. */}
                  <Button
                    variant="outline"
                    size="sm"
                    disabled={t.managed}
                    title={t.managed ? 'Managed by server configuration — edit it there' : undefined}
                    onClick={() => setEditing(t)}
                  >
                    Edit
                  </Button>
                  <Button
                    variant="outline"
                    size="sm"
                    disabled={t.managed || updateMutation.isPending}
                    title={t.managed ? 'Managed by server configuration — edit it there' : undefined}
                    onClick={() =>
                      updateMutation.mutate({ slug: t.slug, body: { active: !t.active } })
                    }
                  >
                    {t.active ? 'Deactivate' : 'Activate'}
                  </Button>
                  <Button
                    variant="destructive"
                    size="sm"
                    disabled={t.managed}
                    title={t.managed ? 'Managed by server configuration — remove it there' : undefined}
                    onClick={() => setDeleting(t)}
                  >
                    Delete
                  </Button>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}

      {editing && (
        <EditTagDialog
          tag={editing}
          isPending={updateMutation.isPending}
          onCancel={() => setEditing(null)}
          onSave={(body) => updateMutation.mutate({ slug: editing.slug, body })}
        />
      )}

      <ConfirmDialog
        open={deleting !== null}
        onOpenChange={(open) => {
          if (!open) setDeleting(null)
        }}
        title={`Delete tag "${deleting?.name ?? ''}"?`}
        description="Only possible while no version carries this tag. If any does, the registry refuses and the tag should be deactivated instead."
        confirmLabel="Delete"
        destructive
        isPending={deleteMutation.isPending}
        onConfirm={() => {
          if (deleting) deleteMutation.mutate(deleting.slug)
        }}
      />
    </div>
  )
}

interface EditTagDialogProps {
  tag: InstanceTag
  isPending: boolean
  onCancel: () => void
  onSave: (body: components['schemas']['UpdateInstanceTagRequest']) => void
}

function EditTagDialog({ tag, isPending, onCancel, onSave }: EditTagDialogProps) {
  const [name, setName] = useState(tag.name)
  const [description, setDescription] = useState(tag.description ?? '')
  const [color, setColor] = useState<TagColor>((tag.color as TagColor | undefined) ?? 'gray')

  return (
    <div className="rounded-lg border p-4 space-y-3">
      <h2 className="font-semibold">
        Edit tag <span className="font-mono text-sm">{tag.slug}</span>
      </h2>
      <p className="text-sm text-muted-foreground">
        The slug is frozen on published versions and cannot change; display
        name, description, and color apply everywhere immediately.
      </p>
      <form
        className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4"
        onSubmit={(e) => {
          e.preventDefault()
          onSave({ name, description, color })
        }}
      >
        <div className="space-y-1.5">
          <Label htmlFor="edit-tag-name">Display name</Label>
          <Input
            id="edit-tag-name"
            name="name"
            value={name}
            onChange={(e) => setName(e.target.value)}
            required
          />
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="edit-tag-description">Description</Label>
          <Input
            id="edit-tag-description"
            name="description"
            value={description}
            onChange={(e) => setDescription(e.target.value)}
          />
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="edit-tag-color">Color</Label>
          <NativeSelect
            id="edit-tag-color"
            name="color"
            className="w-full"
            value={color}
            onChange={(e) => setColor(e.target.value as TagColor)}
          >
            {TAG_COLORS.map((c) => (
              <option key={c} value={c}>
                {c}
              </option>
            ))}
          </NativeSelect>
        </div>
        <div className="flex items-end gap-2">
          <Button type="submit" disabled={isPending}>
            Save
          </Button>
          <Button type="button" variant="outline" onClick={onCancel}>
            Cancel
          </Button>
        </div>
      </form>
    </div>
  )
}
