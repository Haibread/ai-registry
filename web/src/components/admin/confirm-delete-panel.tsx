import { useState, type ReactNode } from 'react'
import { AlertTriangle } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

interface ConfirmDeletePanelProps {
  // The exact text the operator must type to arm the delete — typically the
  // entity's name. Matching is case- and whitespace-sensitive on purpose.
  confirmText: string
  // Human label for the entity kind, woven into the copy (e.g. "publisher").
  entityLabel: string
  // Optional impact summary shown above the input — e.g. the cascade of
  // resources that will also be removed.
  summary?: ReactNode
  onConfirm: () => void
  onCancel: () => void
  isPending?: boolean
  // Error message shown when a delete attempt fails.
  error?: string
}

// An inline "danger zone" confirmation for irreversible, cascading deletes.
// The submit button stays disabled until the operator types `confirmText`
// verbatim, so a stray click can't wipe a populated entity.
export function ConfirmDeletePanel({
  confirmText,
  entityLabel,
  summary,
  onConfirm,
  onCancel,
  isPending,
  error,
}: ConfirmDeletePanelProps) {
  const [typed, setTyped] = useState('')
  const matches = typed === confirmText

  return (
    <form
      className="space-y-4 border border-destructive/40 rounded-lg p-4 max-w-sm"
      onSubmit={(e) => {
        e.preventDefault()
        if (matches && !isPending) onConfirm()
      }}
    >
      <div className="flex items-center gap-2 text-destructive">
        <AlertTriangle className="h-4 w-4" aria-hidden="true" />
        <h3 className="text-base font-semibold">Delete {entityLabel}</h3>
      </div>

      {summary}

      <div className="space-y-1">
        <Label htmlFor="confirm-delete-input">
          Type{' '}
          <span className="font-mono font-medium text-foreground break-all">
            {confirmText}
          </span>{' '}
          to confirm
        </Label>
        <Input
          id="confirm-delete-input"
          value={typed}
          onChange={(e) => setTyped(e.target.value)}
          autoComplete="off"
          autoCapitalize="off"
          spellCheck={false}
          aria-invalid={typed.length > 0 && !matches}
        />
      </div>

      {error && (
        <p role="alert" className="text-sm text-destructive">
          {error}
        </p>
      )}

      <div className="flex gap-2">
        <Button
          type="submit"
          variant="destructive"
          size="sm"
          disabled={!matches || isPending}
        >
          {isPending ? 'Deleting…' : `Delete ${entityLabel}`}
        </Button>
        <Button type="button" variant="outline" size="sm" onClick={onCancel}>
          Cancel
        </Button>
      </div>
    </form>
  )
}
