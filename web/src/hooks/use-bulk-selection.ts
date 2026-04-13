import { useState, useCallback } from 'react'

/**
 * useBulkSelection — manage a set of selected IDs for bulk actions.
 */
export function useBulkSelection<T extends { id: string }>() {
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set())

  const toggle = useCallback((id: string) => {
    setSelectedIds((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }, [])

  const toggleAll = useCallback((items: T[]) => {
    setSelectedIds((prev) => {
      if (prev.size === items.length && items.every((i) => prev.has(i.id))) {
        return new Set()
      }
      return new Set(items.map((i) => i.id))
    })
  }, [])

  const clear = useCallback(() => setSelectedIds(new Set()), [])

  const isSelected = useCallback((id: string) => selectedIds.has(id), [selectedIds])

  return {
    selectedIds,
    selectedCount: selectedIds.size,
    toggle,
    toggleAll,
    clear,
    isSelected,
  }
}
