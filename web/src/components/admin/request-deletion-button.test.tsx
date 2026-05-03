import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, waitFor, fireEvent } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'

const mockPOST = vi.fn()
vi.mock('@/lib/api-client', () => ({
  useAuthClient: () => ({ POST: mockPOST }),
}))

import { RequestDeletionButton } from './request-deletion-button'

function renderButton(kind: 'mcp' | 'agent' = 'mcp') {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <RequestDeletionButton kind={kind} namespace="acme" slug="weather" entityName="Weather" />
    </QueryClientProvider>,
  )
}

describe('RequestDeletionButton', () => {
  // ReturnType<typeof vi.spyOn<Window, 'confirm'>> trips the strict
  // function-type assignability check; an `any` type for the spy
  // handle is the simplest escape hatch and only loses the typed
  // helpers like mockReturnValue (which we still call below — vitest
  // accepts them at runtime).
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  let confirmSpy: any

  beforeEach(() => {
    vi.clearAllMocks()
    mockPOST.mockResolvedValue({})
    confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true)
  })

  afterEach(() => {
    confirmSpy.mockRestore()
  })

  it('posts to the mcp deletion-request endpoint when kind=mcp', async () => {
    renderButton('mcp')
    fireEvent.click(screen.getByRole('button', { name: /request deletion/i }))
    await waitFor(() => {
      expect(mockPOST).toHaveBeenCalledWith(
        '/api/v1/mcp/servers/{namespace}/{slug}/deletion-request',
        { params: { path: { namespace: 'acme', slug: 'weather' } } },
      )
    })
  })

  it('posts to the agent endpoint when kind=agent', async () => {
    renderButton('agent')
    fireEvent.click(screen.getByRole('button', { name: /request deletion/i }))
    await waitFor(() => {
      expect(mockPOST).toHaveBeenCalledWith(
        '/api/v1/agents/{namespace}/{slug}/deletion-request',
        { params: { path: { namespace: 'acme', slug: 'weather' } } },
      )
    })
  })

  it('does nothing when the user cancels the confirm prompt', () => {
    confirmSpy.mockReturnValue(false)
    renderButton()
    fireEvent.click(screen.getByRole('button', { name: /request deletion/i }))
    expect(mockPOST).not.toHaveBeenCalled()
  })

  it('shows success copy and disables itself after a 202', async () => {
    renderButton()
    fireEvent.click(screen.getByRole('button', { name: /request deletion/i }))
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
    fireEvent.click(screen.getByRole('button', { name: /request deletion/i }))
    expect(
      await screen.findByText(/deletion is already pending review/i),
    ).toBeInTheDocument()
  })

  it('falls through to the server detail message on other errors', async () => {
    mockPOST.mockResolvedValue({
      error: { status: 500, detail: 'database hiccup' },
    })
    renderButton()
    fireEvent.click(screen.getByRole('button', { name: /request deletion/i }))
    expect(await screen.findByText(/database hiccup/i)).toBeInTheDocument()
  })
})
