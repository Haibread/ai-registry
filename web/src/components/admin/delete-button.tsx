import { Button } from '@/components/ui/button'

interface DeleteButtonProps {
  onDelete: () => void
  entityName: string
  isPending?: boolean
}

export function DeleteButton({ onDelete, entityName, isPending }: DeleteButtonProps) {
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
      Delete
    </Button>
  )
}
