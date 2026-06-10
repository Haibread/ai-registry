import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, within } from '@testing-library/react'
import { DeleteButton } from './delete-button'

// JSDOM doesn't implement HTMLDialogElement's modal methods; stub them so the
// ConfirmDialog opens and closes.
beforeEach(() => {
  if (!HTMLDialogElement.prototype.showModal) {
    HTMLDialogElement.prototype.showModal = function () {
      this.setAttribute('open', '')
    }
  }
  if (!HTMLDialogElement.prototype.close) {
    HTMLDialogElement.prototype.close = function () {
      this.removeAttribute('open')
      this.dispatchEvent(new Event('close'))
    }
  }
})

describe('DeleteButton', () => {
  it('renders a solid destructive Delete button', () => {
    render(<DeleteButton onDelete={() => {}} entityName="acme/bot" />)
    const btn = screen.getByRole('button', { name: /delete/i })
    expect(btn).toBeInTheDocument()
    // Irreversible break-glass action — its weight matches its blast radius.
    expect(btn.className).toContain('destructive')
  })

  it('calls onDelete after the dialog naming the entity is confirmed', () => {
    const onDelete = vi.fn()
    render(<DeleteButton onDelete={onDelete} entityName="acme/bot" />)
    fireEvent.click(screen.getByRole('button', { name: /delete/i }))
    expect(onDelete).not.toHaveBeenCalled()
    expect(screen.getByRole('heading', { name: /delete "acme\/bot"\?/i })).toBeInTheDocument()
    const dialog = document.querySelector('dialog')!
    fireEvent.click(within(dialog).getByRole('button', { name: /^delete$/i }))
    expect(onDelete).toHaveBeenCalledOnce()
  })

  it('does not call onDelete when the user cancels', () => {
    const onDelete = vi.fn()
    render(<DeleteButton onDelete={onDelete} entityName="acme/bot" />)
    fireEvent.click(screen.getByRole('button', { name: /delete/i }))
    fireEvent.click(screen.getByRole('button', { name: /^cancel$/i }))
    expect(onDelete).not.toHaveBeenCalled()
  })

  it('disables the button when isPending is true', () => {
    render(<DeleteButton onDelete={() => {}} entityName="acme/bot" isPending />)
    expect(screen.getByRole('button', { name: /delete/i })).toBeDisabled()
  })
})
