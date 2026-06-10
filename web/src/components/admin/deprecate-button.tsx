import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { ConfirmDialog } from '@/components/ui/confirm-dialog'

interface DeprecateButtonProps {
  onDeprecate: () => void
  entityName: string
}

// Deprecation is reversible (Republish / undeprecate), so the trigger is a
// quiet outline button — destructive weight is reserved for actions that
// actually destroy something (see DeleteButton).
export function DeprecateButton({ onDeprecate, entityName }: DeprecateButtonProps) {
  const [confirming, setConfirming] = useState(false)

  return (
    <>
      <Button type="button" variant="outline" size="sm" onClick={() => setConfirming(true)}>
        Deprecate
      </Button>
      <ConfirmDialog
        open={confirming}
        onOpenChange={setConfirming}
        title={`Deprecate "${entityName}"?`}
        description="This marks the entry as deprecated, signalling consumers to migrate away. It can be republished later."
        confirmLabel="Deprecate"
        onConfirm={() => {
          onDeprecate()
          setConfirming(false)
        }}
      />
    </>
  )
}
