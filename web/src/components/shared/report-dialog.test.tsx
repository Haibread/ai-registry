import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { ReportDialog } from './report-dialog'

const mockPOST = vi.fn()
vi.mock('@/lib/api-client', () => ({
  getPublicClient: () => ({ POST: mockPOST }),
}))

// JSDOM doesn't implement HTMLDialogElement's modal methods; stub them.
beforeEach(() => {
  mockPOST.mockReset()
  mockPOST.mockResolvedValue({})
  if (!HTMLDialogElement.prototype.showModal) {
    HTMLDialogElement.prototype.showModal = function () {
      this.setAttribute('open', '')
    }
  }
  if (!HTMLDialogElement.prototype.close) {
    HTMLDialogElement.prototype.close = function () {
      this.removeAttribute('open')
    }
  }
})

describe('ReportDialog', () => {
  it('renders the trigger button', () => {
    render(<ReportDialog resourceType="mcp_server" resourceId="01H" resourceLabel="acme/srv" />)
    expect(screen.getByRole('button', { name: /report an issue/i })).toBeInTheDocument()
  })

  it('opens the dialog when the trigger is clicked', () => {
    render(<ReportDialog resourceType="mcp_server" resourceId="01H" resourceLabel="acme/srv" />)
    fireEvent.click(screen.getByRole('button', { name: /report an issue/i }))
    expect(screen.getByRole('heading', { name: /report an issue/i })).toBeInTheDocument()
    expect(screen.getByText('acme/srv')).toBeInTheDocument()
  })

  it('rejects too-short descriptions', async () => {
    render(<ReportDialog resourceType="agent" resourceId="01H" />)
    fireEvent.click(screen.getByRole('button', { name: /report an issue/i }))
    const textarea = screen.getByLabelText(/description/i)
    fireEvent.change(textarea, { target: { value: 'hi' } })
    fireEvent.click(screen.getByRole('button', { name: /submit report/i }))
    await waitFor(() => {
      expect(screen.getByRole('alert')).toHaveTextContent(/at least 5 characters/i)
    })
    expect(mockPOST).not.toHaveBeenCalled()
  })

  it('submits a valid report and shows success state', async () => {
    render(<ReportDialog resourceType="mcp_server" resourceId="01HMCP" />)
    fireEvent.click(screen.getByRole('button', { name: /report an issue/i }))
    fireEvent.change(screen.getByLabelText(/issue type/i), { target: { value: 'spam' } })
    fireEvent.change(screen.getByLabelText(/description/i), {
      target: { value: 'this is clearly spam advertising' },
    })
    fireEvent.click(screen.getByRole('button', { name: /submit report/i }))
    await waitFor(() => {
      expect(mockPOST).toHaveBeenCalledWith('/api/v1/reports', {
        body: {
          resource_type: 'mcp_server',
          resource_id: '01HMCP',
          issue_type: 'spam',
          description: 'this is clearly spam advertising',
        },
      })
    })
    await waitFor(() => {
      expect(screen.getByText(/submitted for review/i)).toBeInTheDocument()
    })
  })

  it('shows API error messages when submission fails', async () => {
    mockPOST.mockResolvedValueOnce({ error: { detail: 'rate limited' } })
    render(<ReportDialog resourceType="agent" resourceId="01HAG" />)
    fireEvent.click(screen.getByRole('button', { name: /report an issue/i }))
    fireEvent.change(screen.getByLabelText(/description/i), {
      target: { value: 'a proper description here' },
    })
    fireEvent.click(screen.getByRole('button', { name: /submit report/i }))
    await waitFor(() => {
      expect(screen.getByRole('alert')).toHaveTextContent(/rate limited/i)
    })
  })
})
