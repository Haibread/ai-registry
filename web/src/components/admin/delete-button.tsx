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
      variant="destructive"
      size="sm"
      disabled={isPending}
      onClick={handleClick}
    >
      {label}
    </Button>
  )
}
