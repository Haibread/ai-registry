import { useState } from 'react'
import { Trash2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { ConfirmDialog } from '@/components/ui/confirm-dialog'

interface DeleteButtonProps {
  onDelete: () => void
  entityName: string
  isPending?: boolean
  // Optional override for the button label. Defaults to "Delete".
  // Useful when more than one DeleteButton appears on the same page so
  // accessible-name selectors (getByRole('button', { name: 'Delete' }))
  // don't trip strict-mode violations.
  label?: string
}

// Solid destructive styling: this is the irreversible break-glass path
// (admin force-delete, bypassing the review workflow), so its weight matches
// its blast radius — unlike the reversible Deprecate, which stays quiet. The
// previous muted-red outline read as disabled (UI/UX review P2.1/P3).
export function DeleteButton({
  onDelete,
  entityName,
  isPending,
  label = 'Delete',
}: DeleteButtonProps) {
  const [confirming, setConfirming] = useState(false)

  return (
    <>
      <Button
        type="button"
        variant="destructive"
        size="sm"
        disabled={isPending}
        onClick={() => setConfirming(true)}
      >
        <Trash2 className="h-3.5 w-3.5" aria-hidden="true" />
        {label}
      </Button>
      <ConfirmDialog
        open={confirming}
        onOpenChange={setConfirming}
        title={`Delete "${entityName}"?`}
        description="This permanently deletes the entry and all its versions, bypassing the review workflow. It cannot be undone."
        confirmLabel="Delete"
        destructive
        isPending={isPending ?? false}
        onConfirm={() => {
          onDelete()
          setConfirming(false)
        }}
      />
    </>
  )
}
