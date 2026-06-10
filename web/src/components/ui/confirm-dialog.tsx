/**
 * ConfirmDialog — the shared confirmation primitive for consequential actions
 * (UI/UX review P1.4/P2.1). A themed, keyboard-accessible replacement for
 * window.confirm: native <dialog> (focus trap + Esc for free), named action
 * and consequence, destructive styling when warranted.
 *
 * Controlled: the caller owns `open` and renders the dialog near the
 * triggering button. `m-auto` is load-bearing — Tailwind's preflight resets
 * the margin that normally centers a modal <dialog> (see review finding J3b).
 */

import { useEffect, useRef } from 'react'
import { Button } from '@/components/ui/button'

interface ConfirmDialogProps {
  open: boolean
  /** Called with false when the dialog closes (Cancel, Esc, backdrop). */
  onOpenChange: (open: boolean) => void
  /** Short imperative question, e.g. `Disable account "x@y"?` */
  title: string
  /** The consequence, spelled out. */
  description?: React.ReactNode
  confirmLabel?: string
  cancelLabel?: string
  /** Style the confirm button as destructive (red). */
  destructive?: boolean
  /** Disables the confirm button while the mutation runs. */
  isPending?: boolean
  onConfirm: () => void
}

export function ConfirmDialog({
  open,
  onOpenChange,
  title,
  description,
  confirmLabel = 'Confirm',
  cancelLabel = 'Cancel',
  destructive = false,
  isPending = false,
  onConfirm,
}: ConfirmDialogProps) {
  const ref = useRef<HTMLDialogElement>(null)

  useEffect(() => {
    const el = ref.current
    if (!el) return
    if (open && !el.open) el.showModal()
    else if (!open && el.open) el.close()
  }, [open])

  return (
    <dialog
      ref={ref}
      onClose={() => onOpenChange(false)}
      onClick={(e) => {
        // A click on the backdrop hits the <dialog> element itself.
        if (e.target === ref.current) onOpenChange(false)
      }}
      className="m-auto w-full max-w-md rounded-lg border bg-background p-6 text-foreground shadow-lg backdrop:bg-black/50"
    >
      {/* Content mounts only while open so a closed dialog can't leak
          duplicate button names into the accessibility tree. */}
      {open && (
        <>
          <h2 className="text-lg font-semibold">{title}</h2>
          {description && <div className="mt-2 text-sm text-muted-foreground">{description}</div>}
          <div className="mt-5 flex justify-end gap-2">
            <Button type="button" variant="outline" size="sm" onClick={() => onOpenChange(false)}>
              {cancelLabel}
            </Button>
            <Button
              type="button"
              variant={destructive ? 'destructive' : 'default'}
              size="sm"
              disabled={isPending}
              onClick={onConfirm}
            >
              {confirmLabel}
            </Button>
          </div>
        </>
      )}
    </dialog>
  )
}
