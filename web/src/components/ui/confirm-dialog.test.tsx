import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { ConfirmDialog } from './confirm-dialog'

// JSDOM doesn't implement HTMLDialogElement's modal methods; stub them.
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

function renderDialog(overrides: Partial<React.ComponentProps<typeof ConfirmDialog>> = {}) {
  const onConfirm = vi.fn()
  const onOpenChange = vi.fn()
  render(
    <ConfirmDialog
      open
      onOpenChange={onOpenChange}
      title='Disable account "a@x.test"?'
      description="They will not be able to sign in."
      confirmLabel="Disable account"
      destructive
      onConfirm={onConfirm}
      {...overrides}
    />,
  )
  return { onConfirm, onOpenChange }
}

describe('ConfirmDialog', () => {
  it('renders title, description, and both buttons when open', () => {
    renderDialog()
    expect(screen.getByRole('heading', { name: /disable account "a@x.test"\?/i })).toBeInTheDocument()
    expect(screen.getByText(/they will not be able to sign in/i)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /^cancel$/i })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /^disable account$/i })).toBeInTheDocument()
  })

  it('renders nothing while closed', () => {
    renderDialog({ open: false })
    expect(screen.queryByRole('heading')).not.toBeInTheDocument()
    expect(screen.queryByRole('button')).not.toBeInTheDocument()
  })

  it('fires onConfirm from the confirm button', () => {
    const { onConfirm } = renderDialog()
    fireEvent.click(screen.getByRole('button', { name: /^disable account$/i }))
    expect(onConfirm).toHaveBeenCalledOnce()
  })

  it('closes via Cancel without confirming', () => {
    const { onConfirm, onOpenChange } = renderDialog()
    fireEvent.click(screen.getByRole('button', { name: /^cancel$/i }))
    expect(onOpenChange).toHaveBeenCalledWith(false)
    expect(onConfirm).not.toHaveBeenCalled()
  })

  it('reports Esc-close back through onOpenChange', () => {
    const { onOpenChange } = renderDialog()
    // Native <dialog> turns Esc into a close event; simulate it directly.
    fireEvent(document.querySelector('dialog')!, new Event('close'))
    expect(onOpenChange).toHaveBeenCalledWith(false)
  })

  it('disables the confirm button while pending', () => {
    renderDialog({ isPending: true })
    expect(screen.getByRole('button', { name: /^disable account$/i })).toBeDisabled()
  })
})
