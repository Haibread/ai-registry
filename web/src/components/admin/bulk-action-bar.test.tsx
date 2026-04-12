import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { BulkActionBar } from './bulk-action-bar'

function noop() {}

describe('BulkActionBar', () => {
  it('renders nothing when selectedCount is 0', () => {
    const { container } = render(
      <BulkActionBar
        selectedCount={0}
        onClear={noop}
        onSetVisibility={noop}
        onDeprecate={noop}
        onDelete={noop}
      />,
    )
    expect(container.innerHTML).toBe('')
  })

  it('shows selected count when > 0', () => {
    render(
      <BulkActionBar
        selectedCount={3}
        onClear={noop}
        onSetVisibility={noop}
        onDeprecate={noop}
        onDelete={noop}
      />,
    )
    expect(screen.getByText(/3 selected/i)).toBeInTheDocument()
  })

  it('calls onClear when clear button is clicked', () => {
    const onClear = vi.fn()
    render(
      <BulkActionBar
        selectedCount={2}
        onClear={onClear}
        onSetVisibility={noop}
        onDeprecate={noop}
        onDelete={noop}
      />,
    )
    fireEvent.click(screen.getByLabelText(/clear selection/i))
    expect(onClear).toHaveBeenCalled()
  })

  it('calls onSetVisibility with public', () => {
    const onSetVisibility = vi.fn()
    render(
      <BulkActionBar
        selectedCount={1}
        onClear={noop}
        onSetVisibility={onSetVisibility}
        onDeprecate={noop}
        onDelete={noop}
      />,
    )
    fireEvent.click(screen.getByText('Public'))
    expect(onSetVisibility).toHaveBeenCalledWith('public')
  })

  it('calls onSetVisibility with private', () => {
    const onSetVisibility = vi.fn()
    render(
      <BulkActionBar
        selectedCount={1}
        onClear={noop}
        onSetVisibility={onSetVisibility}
        onDeprecate={noop}
        onDelete={noop}
      />,
    )
    fireEvent.click(screen.getByText('Private'))
    expect(onSetVisibility).toHaveBeenCalledWith('private')
  })

  it('calls onDeprecate', () => {
    const onDeprecate = vi.fn()
    render(
      <BulkActionBar
        selectedCount={1}
        onClear={noop}
        onSetVisibility={noop}
        onDeprecate={onDeprecate}
        onDelete={noop}
      />,
    )
    fireEvent.click(screen.getByText('Deprecate'))
    expect(onDeprecate).toHaveBeenCalled()
  })

  it('calls onDelete', () => {
    const onDelete = vi.fn()
    render(
      <BulkActionBar
        selectedCount={1}
        onClear={noop}
        onSetVisibility={noop}
        onDeprecate={noop}
        onDelete={onDelete}
      />,
    )
    fireEvent.click(screen.getByText('Delete'))
    expect(onDelete).toHaveBeenCalled()
  })

  it('disables action buttons when isBusy', () => {
    render(
      <BulkActionBar
        selectedCount={1}
        onClear={noop}
        onSetVisibility={noop}
        onDeprecate={noop}
        onDelete={noop}
        isBusy
      />,
    )
    expect(screen.getByText('Public').closest('button')).toBeDisabled()
    expect(screen.getByText('Delete').closest('button')).toBeDisabled()
  })
})
