/**
 * BulkActionBar — floating action bar shown when one or more rows are selected
 * on an admin list page. Provides visibility toggles, deprecate, and delete.
 */

import { Button } from '@/components/ui/button'
import { X, Eye, EyeOff, AlertTriangle, Trash2 } from 'lucide-react'

interface BulkActionBarProps {
  selectedCount: number
  onClear: () => void
  onSetVisibility: (visibility: 'public' | 'private') => void
  onDeprecate: () => void
  onDelete: () => void
  isBusy?: boolean
  // Role-gating: the bar only renders the actions the caller can
  // perform. Visibility flips and deletes are Server-Admin-only; deprecate is a
  // publisher Editor action. All default to true for backwards compatibility.
  canSetVisibility?: boolean
  canDeprecate?: boolean
  canDelete?: boolean
}

export function BulkActionBar({
  selectedCount,
  onClear,
  onSetVisibility,
  onDeprecate,
  onDelete,
  isBusy,
  canSetVisibility = true,
  canDeprecate = true,
  canDelete = true,
}: BulkActionBarProps) {
  if (selectedCount === 0) return null

  return (
    <div
      role="toolbar"
      aria-label="Bulk actions"
      className="fixed bottom-6 left-1/2 -translate-x-1/2 z-40 flex items-center gap-2 rounded-full border bg-background/95 backdrop-blur-sm px-3 py-2 shadow-lg max-w-[calc(100vw-1.5rem)] overflow-x-auto"
    >
      <Button variant="ghost" size="sm" onClick={onClear} aria-label="Clear selection">
        <X className="h-4 w-4" />
      </Button>
      <span className="text-sm font-medium px-1">
        {selectedCount} selected
      </span>
      {canSetVisibility && (
        <>
          <div className="h-4 w-px bg-border" />
          <Button
            variant="ghost"
            size="sm"
            onClick={() => onSetVisibility('public')}
            disabled={isBusy}
            className="gap-1.5"
          >
            <Eye className="h-3.5 w-3.5" /> Public
          </Button>
          <Button
            variant="ghost"
            size="sm"
            onClick={() => onSetVisibility('private')}
            disabled={isBusy}
            className="gap-1.5"
          >
            <EyeOff className="h-3.5 w-3.5" /> Private
          </Button>
        </>
      )}
      {canDeprecate && (
        <>
          <div className="h-4 w-px bg-border" />
          <Button
            variant="ghost"
            size="sm"
            onClick={onDeprecate}
            disabled={isBusy}
            className="gap-1.5 text-yellow-700 dark:text-yellow-500"
          >
            <AlertTriangle className="h-3.5 w-3.5" /> Deprecate
          </Button>
        </>
      )}
      {canDelete && (
        <Button
          variant="ghost"
          size="sm"
          onClick={onDelete}
          disabled={isBusy}
          className="gap-1.5 text-destructive"
        >
          <Trash2 className="h-3.5 w-3.5" /> Delete
        </Button>
      )}
    </div>
  )
}
