import { Button } from '@/components/ui/button'

interface DeprecateButtonProps {
  onDeprecate: () => void
  entityName: string
}

export function DeprecateButton({ onDeprecate, entityName }: DeprecateButtonProps) {
  function handleClick() {
    const confirmed = window.confirm(
      `Deprecate "${entityName}"?\n\nThis marks the entry as deprecated. This action cannot be undone.`
    )
    if (confirmed) onDeprecate()
  }

  return (
    <Button type="button" variant="destructive" size="sm" onClick={handleClick}>
      Deprecate
    </Button>
  )
}
