import { Trash2 } from 'lucide-react'
import { Button } from '@/components/ui/button'

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

// Quiet "destructive outline" styling: red text + red border at low opacity,
// filled red on hover. This keeps Delete readable as destructive without
// drowning out the row's primary actions (Edit / Manage). The window.confirm
// gate is the actual safety net, so the button doesn't need to scream.
export function DeleteButton({
  onDelete,
  entityName,
  isPending,
  label = 'Delete',
}: DeleteButtonProps) {
  function handleClick() {
    const confirmed = window.confirm(
      `Delete "${entityName}"?\n\nThis action cannot be undone.`
    )
    if (confirmed) onDelete()
  }

  return (
    <Button
      type="button"
      variant="outline"
      size="sm"
      disabled={isPending}
      onClick={handleClick}
      className="border-destructive/40 text-destructive hover:bg-destructive hover:text-destructive-foreground hover:border-destructive"
    >
      <Trash2 className="h-3.5 w-3.5" aria-hidden="true" />
      {label}
    </Button>
  )
}
