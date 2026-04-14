import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { DeleteButton } from './delete-button'

describe('DeleteButton', () => {
  let confirmSpy: ReturnType<typeof vi.spyOn>

  beforeEach(() => {
    confirmSpy = vi.spyOn(window, 'confirm')
  })

  afterEach(() => {
    confirmSpy.mockRestore()
  })

  it('renders a Delete button', () => {
    render(<DeleteButton onDelete={() => {}} entityName="acme/bot" />)
    expect(screen.getByRole('button', { name: /delete/i })).toBeInTheDocument()
  })

  it('calls onDelete when the user confirms', () => {
    confirmSpy.mockReturnValue(true)
    const onDelete = vi.fn()
    render(<DeleteButton onDelete={onDelete} entityName="acme/bot" />)
    fireEvent.click(screen.getByRole('button', { name: /delete/i }))
    expect(confirmSpy).toHaveBeenCalledWith(expect.stringContaining('acme/bot'))
    expect(onDelete).toHaveBeenCalledOnce()
  })

  it('does not call onDelete when the user cancels', () => {
    confirmSpy.mockReturnValue(false)
    const onDelete = vi.fn()
    render(<DeleteButton onDelete={onDelete} entityName="acme/bot" />)
    fireEvent.click(screen.getByRole('button', { name: /delete/i }))
    expect(onDelete).not.toHaveBeenCalled()
  })

  it('disables the button when isPending is true', () => {
    render(<DeleteButton onDelete={() => {}} entityName="acme/bot" isPending />)
    expect(screen.getByRole('button', { name: /delete/i })).toBeDisabled()
  })
})
