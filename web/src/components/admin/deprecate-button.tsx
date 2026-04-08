"use client"

import { useRef } from "react"
import { Button } from "@/components/ui/button"

interface DeprecateButtonProps {
  action: () => Promise<void>
  entityName: string
}

/**
 * Wraps a server action for deprecation behind a confirmation dialog.
 * Deprecation is irreversible, so we require explicit user confirmation.
 */
export function DeprecateButton({ action, entityName }: DeprecateButtonProps) {
  const formRef = useRef<HTMLFormElement>(null)

  function handleClick(e: React.MouseEvent) {
    e.preventDefault()
    const confirmed = window.confirm(
      `Deprecate "${entityName}"?\n\nThis marks the entry as deprecated. This action cannot be undone.`
    )
    if (confirmed) {
      formRef.current?.requestSubmit()
    }
  }

  return (
    <form ref={formRef} action={action}>
      <Button
        type="button"
        variant="destructive"
        size="sm"
        onClick={handleClick}
      >
        Deprecate
      </Button>
    </form>
  )
}
