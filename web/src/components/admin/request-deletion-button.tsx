import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { Trash2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { useAuthClient } from '@/lib/api-client'

interface RequestDeletionButtonProps {
  kind: 'mcp' | 'agent'
  namespace: string
  slug: string
  entityName: string
}

// Surface for the deletion-request workflow: publisher Editors
// (and admins) submit a deletion through review here. The reviewer
// approves or rejects from /admin/review.
//
// We can't know up-front whether a deletion is already pending without
// adding deletion_requested_at to the entry GET response, so the
// button always tries the POST and maps 409 / 4xx to friendly messages.
export function RequestDeletionButton({
  kind,
  namespace,
  slug,
  entityName,
}: RequestDeletionButtonProps) {
  const api = useAuthClient()
  const queryClient = useQueryClient()
  const [error, setError] = useState<string | null>(null)
  const [success, setSuccess] = useState(false)

  const mutation = useMutation({
    mutationFn: async () => {
      setError(null)
      setSuccess(false)
      const result =
        kind === 'mcp'
          ? await api.POST(
              '/api/v1/mcp/servers/{namespace}/{slug}/deletion-request',
              { params: { path: { namespace, slug } } },
            )
          : await api.POST(
              '/api/v1/agents/{namespace}/{slug}/deletion-request',
              { params: { path: { namespace, slug } } },
            )
      if (result.error) {
        const e = result.error as { status?: number; type?: string; detail?: string }
        if (e?.status === 409 || e?.type?.includes('review-already-pending') || e?.type?.includes('conflict')) {
          throw new Error('A deletion is already pending review for this entry.')
        }
        throw new Error(e?.detail || 'Could not submit deletion request.')
      }
    },
    onSuccess: () => {
      setSuccess(true)
      toast.success(`Deletion request submitted for ${entityName}`)
      queryClient.invalidateQueries({ queryKey: ['admin-review-queue-count'] })
    },
    onError: (err: Error) => setError(err.message),
  })

  return (
    <div className="space-y-2">
      <Button
        type="button"
        variant="outline"
        size="sm"
        disabled={mutation.isPending || success}
        onClick={() => {
          const ok = window.confirm(
            `Request deletion of "${entityName}"?\n\nA reviewer must approve before the entry is removed.`,
          )
          if (ok) mutation.mutate()
        }}
      >
        <Trash2 className="h-4 w-4" />
        <span className="ml-1.5">
          {success ? 'Pending review' : mutation.isPending ? 'Submitting…' : 'Request deletion (review)'}
        </span>
      </Button>
      {error && (
        <p role="alert" className="text-sm text-destructive">
          {error}
        </p>
      )}
      {success && (
        <p className="text-sm text-muted-foreground">
          Submitted. A reviewer will approve or reject the request from the review queue.
        </p>
      )}
    </div>
  )
}
