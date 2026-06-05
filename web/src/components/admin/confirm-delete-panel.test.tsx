import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { ConfirmDeletePanel } from './confirm-delete-panel'

function renderPanel(overrides: Partial<Parameters<typeof ConfirmDeletePanel>[0]> = {}) {
  const onConfirm = vi.fn()
  const onCancel = vi.fn()
  render(
    <ConfirmDeletePanel
      entityLabel="publisher"
      confirmText="Acme Corp"
      onConfirm={onConfirm}
      onCancel={onCancel}
      {...overrides}
    />,
  )
  return { onConfirm, onCancel }
}

describe('ConfirmDeletePanel', () => {
  it('keeps the confirm button disabled until the text matches exactly', () => {
    renderPanel()
    const submit = screen.getByRole('button', { name: /delete publisher/i })
    expect(submit).toBeDisabled()

    const input = screen.getByLabelText(/to confirm/i)
    fireEvent.change(input, { target: { value: 'acme corp' } }) // wrong case
    expect(submit).toBeDisabled()

    fireEvent.change(input, { target: { value: 'Acme Corp' } })
    expect(submit).toBeEnabled()
  })

  it('calls onConfirm only once the text matches', () => {
    const { onConfirm } = renderPanel()
    const input = screen.getByLabelText(/to confirm/i)

    fireEvent.change(input, { target: { value: 'nope' } })
    fireEvent.submit(input.closest('form') as HTMLFormElement)
    expect(onConfirm).not.toHaveBeenCalled()

    fireEvent.change(input, { target: { value: 'Acme Corp' } })
    fireEvent.click(screen.getByRole('button', { name: /delete publisher/i }))
    expect(onConfirm).toHaveBeenCalledOnce()
  })

  it('calls onCancel from the Cancel button', () => {
    const { onCancel } = renderPanel()
    fireEvent.click(screen.getByRole('button', { name: /cancel/i }))
    expect(onCancel).toHaveBeenCalledOnce()
  })

  it('does not confirm while a delete is pending, even with matching text', () => {
    const { onConfirm } = renderPanel({ isPending: true })
    fireEvent.change(screen.getByLabelText(/to confirm/i), { target: { value: 'Acme Corp' } })
    const submit = screen.getByRole('button', { name: /deleting/i })
    expect(submit).toBeDisabled()
    fireEvent.click(submit)
    expect(onConfirm).not.toHaveBeenCalled()
  })

  it('renders the impact summary and an error message', () => {
    renderPanel({ summary: <p>removes 3 servers</p>, error: 'Delete failed — please try again.' })
    expect(screen.getByText(/removes 3 servers/i)).toBeInTheDocument()
    expect(screen.getByRole('alert')).toHaveTextContent(/delete failed/i)
  })
})
