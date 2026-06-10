import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, within } from '@testing-library/react'
import { DeprecateButton } from './deprecate-button'

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

describe('DeprecateButton', () => {
  it('renders a quiet (non-destructive) Deprecate button', () => {
    render(<DeprecateButton onDeprecate={() => {}} entityName="acme/srv" />)
    const btn = screen.getByRole('button', { name: /deprecate/i })
    expect(btn).toBeInTheDocument()
    // Deprecation is reversible — it must not wear destructive styling.
    expect(btn.className).not.toContain('destructive')
  })

  it('calls onDeprecate after the dialog naming the entity is confirmed', () => {
    const onDeprecate = vi.fn()
    render(<DeprecateButton onDeprecate={onDeprecate} entityName="acme/srv" />)
    fireEvent.click(screen.getByRole('button', { name: /deprecate/i }))
    expect(onDeprecate).not.toHaveBeenCalled()
    expect(screen.getByRole('heading', { name: /deprecate "acme\/srv"\?/i })).toBeInTheDocument()
    // The copy must not claim irreversibility — republish exists.
    expect(screen.getByText(/can be republished later/i)).toBeInTheDocument()
    const dialog = document.querySelector('dialog')!
    fireEvent.click(within(dialog).getByRole('button', { name: /^deprecate$/i }))
    expect(onDeprecate).toHaveBeenCalledOnce()
  })

  it('does not call onDeprecate when the user cancels', () => {
    const onDeprecate = vi.fn()
    render(<DeprecateButton onDeprecate={onDeprecate} entityName="acme/srv" />)
    fireEvent.click(screen.getByRole('button', { name: /deprecate/i }))
    fireEvent.click(screen.getByRole('button', { name: /^cancel$/i }))
    expect(onDeprecate).not.toHaveBeenCalled()
  })
})
