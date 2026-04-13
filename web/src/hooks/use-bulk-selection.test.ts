import { describe, it, expect } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import { useBulkSelection } from './use-bulk-selection'

describe('useBulkSelection', () => {
  it('starts with empty selection', () => {
    const { result } = renderHook(() => useBulkSelection())
    expect(result.current.selectedCount).toBe(0)
    expect(result.current.selectedIds.size).toBe(0)
  })

  it('toggles an id on and off', () => {
    const { result } = renderHook(() => useBulkSelection())
    act(() => result.current.toggle('a'))
    expect(result.current.isSelected('a')).toBe(true)
    expect(result.current.selectedCount).toBe(1)
    act(() => result.current.toggle('a'))
    expect(result.current.isSelected('a')).toBe(false)
    expect(result.current.selectedCount).toBe(0)
  })

  it('tracks multiple ids', () => {
    const { result } = renderHook(() => useBulkSelection())
    act(() => {
      result.current.toggle('a')
      result.current.toggle('b')
      result.current.toggle('c')
    })
    expect(result.current.selectedCount).toBe(3)
  })

  it('clears all selection', () => {
    const { result } = renderHook(() => useBulkSelection())
    act(() => {
      result.current.toggle('a')
      result.current.toggle('b')
    })
    expect(result.current.selectedCount).toBe(2)
    act(() => result.current.clear())
    expect(result.current.selectedCount).toBe(0)
  })

  it('toggleAll selects all items when none are selected', () => {
    const { result } = renderHook(() => useBulkSelection<{ id: string }>())
    const items = [{ id: 'a' }, { id: 'b' }, { id: 'c' }]
    act(() => result.current.toggleAll(items))
    expect(result.current.selectedCount).toBe(3)
  })

  it('toggleAll clears all when all are selected', () => {
    const { result } = renderHook(() => useBulkSelection<{ id: string }>())
    const items = [{ id: 'a' }, { id: 'b' }]
    act(() => result.current.toggleAll(items))
    expect(result.current.selectedCount).toBe(2)
    act(() => result.current.toggleAll(items))
    expect(result.current.selectedCount).toBe(0)
  })
})
