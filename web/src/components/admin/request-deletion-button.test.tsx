import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor, fireEvent } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'

const mockPOST = vi.fn()
vi.mock('@/lib/api-client', () => ({
  useAuthClient: () => ({ POST: mockPOST }),
}))
vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }))

import { RequestDeletionButton } from './request-deletion-button'

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

function renderButton(kind: 'mcp' | 'agent' = 'mcp') {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <RequestDeletionButton kind={kind} namespace="acme" slug="weather" entityName="Weather" />
    </QueryClientProvider>,
  )
}

// Opens the button's confirmation and confirms it. The trigger and the
// dialog confirm share the "Request deletion" name; only the trigger exists
// before the dialog opens, and the dialog confirm is the later of the two.
function clickThroughConfirm() {
  fireEvent.click(screen.getByRole('button', { name: /^request deletion$/i }))
  const confirmButtons = screen.getAllByRole('button', { name: /^request deletion$/i })
  fireEvent.click(confirmButtons[confirmButtons.length - 1])
}

describe('RequestDeletionButton', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockPOST.mockResolvedValue({})
  })

  it('posts to the mcp deletion-request endpoint when kind=mcp', async () => {
    renderButton('mcp')
    clickThroughConfirm()
    await waitFor(() => {
      expect(mockPOST).toHaveBeenCalledWith(
        '/api/v1/mcp/servers/{namespace}/{slug}/deletion-request',
        { params: { path: { namespace: 'acme', slug: 'weather' } } },
      )
    })
  })

  it('posts to the agent endpoint when kind=agent', async () => {
    renderButton('agent')
    clickThroughConfirm()
    await waitFor(() => {
      expect(mockPOST).toHaveBeenCalledWith(
        '/api/v1/agents/{namespace}/{slug}/deletion-request',
        { params: { path: { namespace: 'acme', slug: 'weather' } } },
      )
    })
  })

  it('does nothing when the user cancels the confirmation', () => {
    renderButton()
    fireEvent.click(screen.getByRole('button', { name: /^request deletion$/i }))
    expect(screen.getByRole('heading', { name: /request deletion of "weather"\?/i })).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: /^cancel$/i }))
    expect(mockPOST).not.toHaveBeenCalled()
  })

  it('shows success copy and disables itself after a 202', async () => {
    renderButton()
    clickThroughConfirm()
    expect(await screen.findByText(/pending review/i)).toBeInTheDocument()
    expect(await screen.findByText(/a reviewer will approve or reject/i)).toBeInTheDocument()
    // The button text flips to "Pending review" and is disabled.
    expect(screen.getByRole('button', { name: /pending review/i })).toBeDisabled()
  })

  it('surfaces a friendly error on already-pending', async () => {
    mockPOST.mockResolvedValue({
      error: { status: 409, type: 'https://registry/errors/review-already-pending' },
    })
    renderButton()
    clickThroughConfirm()
    expect(
      await screen.findByText(/deletion is already pending review/i),
    ).toBeInTheDocument()
  })

  it('falls through to the server detail message on other errors', async () => {
    mockPOST.mockResolvedValue({
      error: { status: 500, detail: 'database hiccup' },
    })
    renderButton()
    clickThroughConfirm()
    expect(await screen.findByText(/database hiccup/i)).toBeInTheDocument()
  })
})
