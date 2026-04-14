import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { DeprecateButton } from './deprecate-button'

describe('DeprecateButton', () => {
  let confirmSpy: ReturnType<typeof vi.spyOn>

  beforeEach(() => {
    confirmSpy = vi.spyOn(window, 'confirm')
  })

  afterEach(() => {
    confirmSpy.mockRestore()
  })

  it('renders a Deprecate button', () => {
    render(<DeprecateButton onDeprecate={() => {}} entityName="acme/srv" />)
    expect(screen.getByRole('button', { name: /deprecate/i })).toBeInTheDocument()
  })

  it('calls onDeprecate when confirmed and passes entity name into confirm prompt', () => {
    confirmSpy.mockReturnValue(true)
    const onDeprecate = vi.fn()
    render(<DeprecateButton onDeprecate={onDeprecate} entityName="acme/srv" />)
    fireEvent.click(screen.getByRole('button', { name: /deprecate/i }))
    expect(confirmSpy).toHaveBeenCalledWith(expect.stringContaining('acme/srv'))
    expect(onDeprecate).toHaveBeenCalledOnce()
  })

  it('does not call onDeprecate when the user cancels', () => {
    confirmSpy.mockReturnValue(false)
    const onDeprecate = vi.fn()
    render(<DeprecateButton onDeprecate={onDeprecate} entityName="acme/srv" />)
    fireEvent.click(screen.getByRole('button', { name: /deprecate/i }))
    expect(onDeprecate).not.toHaveBeenCalled()
  })
})
