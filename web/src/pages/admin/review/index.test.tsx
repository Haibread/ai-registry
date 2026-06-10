import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor, fireEvent } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'

vi.mock('@/auth/AuthContext', () => ({
  useAuth: () => ({ accessToken: 'test-token' }),
}))

const mockGET = vi.fn()
const mockPOST = vi.fn()
vi.mock('@/lib/api-client', () => ({
  useAuthClient: () => ({ GET: mockGET, POST: mockPOST }),
}))

import AdminReviewQueue from './index'

// JSDOM doesn't implement HTMLDialogElement's modal methods; stub them so the
// approve confirmation (ConfirmDialog) opens and closes.
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

// Clicks the row's Approve button, then the dialog's confirm button. Approve
// must never fire from a single unconfirmed click (J2).
async function approveRow(index: number, confirmName: RegExp) {
  const approveButtons = await screen.findAllByRole('button', { name: /^approve$/i })
  fireEvent.click(approveButtons[index])
  fireEvent.click(await screen.findByRole('button', { name: confirmName }))
}

function renderPage() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter>
        <AdminReviewQueue />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

const mcpVersionItem = {
  kind: 'mcp_version',
  publisher_slug: 'acme',
  entry_slug: 'weather',
  entry_id: '01HMCP',
  version: '1.0.0',
  revision: 1,
  submitted_at: '2026-04-10T10:00:00Z',
  submitted_by: 'submitter-uuid',
  submitted_by_email: 'submitter@example.com',
}

const agentDeletionItem = {
  kind: 'agent_deletion',
  publisher_slug: 'globex',
  entry_slug: 'planner',
  entry_id: '01HAG',
  submitted_at: '2026-04-10T11:00:00Z',
  submitted_by: 'submitter-uuid',
  submitted_by_email: 'pub@example.com',
}

describe('AdminReviewQueue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockGET.mockResolvedValue({ data: { items: [mcpVersionItem, agentDeletionItem] } })
    mockPOST.mockResolvedValue({})
  })

  it('renders the page heading', () => {
    renderPage()
    expect(screen.getByRole('heading', { name: /review queue/i })).toBeInTheDocument()
  })

  it('fetches the queue on mount', async () => {
    renderPage()
    await waitFor(() => {
      expect(mockGET).toHaveBeenCalledWith('/api/v1/review-queue')
    })
  })

  it('renders one row per item with kind, slug, and version badges', async () => {
    renderPage()
    expect(await screen.findByText('acme/weather')).toBeInTheDocument()
    expect(screen.getByText('globex/planner')).toBeInTheDocument()
    expect(screen.getByText('v1.0.0')).toBeInTheDocument()
    expect(screen.getByText('rev 1')).toBeInTheDocument()
    // Both kind labels appear.
    expect(screen.getByText('MCP version')).toBeInTheDocument()
    expect(screen.getByText('Agent deletion')).toBeInTheDocument()
  })

  it('renders an empty state when nothing is pending', async () => {
    mockGET.mockResolvedValue({ data: { items: [] } })
    renderPage()
    expect(await screen.findByText(/queue is empty/i)).toBeInTheDocument()
  })

  it('approves a version item via the typed approve endpoint, after a confirm naming it', async () => {
    renderPage()
    const approveButtons = await screen.findAllByRole('button', { name: /^approve$/i })
    fireEvent.click(approveButtons[0])
    // Nothing fires until the dialog (naming entry + version) is confirmed.
    expect(mockPOST).not.toHaveBeenCalled()
    expect(screen.getByRole('heading', { name: /approve acme\/weather v1\.0\.0\?/i })).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: /^approve & publish$/i }))
    await waitFor(() => {
      expect(mockPOST).toHaveBeenCalledWith(
        '/api/v1/mcp/servers/{namespace}/{slug}/versions/{version}/approve',
        {
          params: { path: { namespace: 'acme', slug: 'weather', version: '1.0.0' } },
          body: { revision: 1 },
        },
      )
    })
  })

  it('approves a deletion item via the deletion-request approve endpoint, after a destructive confirm', async () => {
    renderPage()
    const approveButtons = await screen.findAllByRole('button', { name: /^approve$/i })
    // Second row is the agent deletion.
    fireEvent.click(approveButtons[1])
    expect(mockPOST).not.toHaveBeenCalled()
    expect(screen.getByRole('heading', { name: /approve deletion of globex\/planner\?/i })).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: /^approve deletion$/i }))
    await waitFor(() => {
      expect(mockPOST).toHaveBeenCalledWith(
        '/api/v1/agents/{namespace}/{slug}/deletion-request/approve',
        { params: { path: { namespace: 'globex', slug: 'planner' } } },
      )
    })
  })

  it('cancelling the approve confirmation fires nothing', async () => {
    renderPage()
    const approveButtons = await screen.findAllByRole('button', { name: /^approve$/i })
    fireEvent.click(approveButtons[0])
    fireEvent.click(screen.getByRole('button', { name: /^cancel$/i }))
    expect(mockPOST).not.toHaveBeenCalled()
  })

  it('lazily loads and shows a version item\'s submitted content on expand', async () => {
    mockGET.mockImplementation((path: string) =>
      path === '/api/v1/review-queue'
        ? Promise.resolve({ data: { items: [mcpVersionItem, agentDeletionItem] } })
        : Promise.resolve({
            data: {
              version: '1.0.0',
              runtime: 'stdio',
              protocol_version: '2025-03-26',
              packages: [{ registryType: 'npm', identifier: '@acme/weather', version: '1.0.0', transport: { type: 'stdio' } }],
              tools: [{ name: 'get_forecast' }, { name: 'get_alerts' }],
            },
          }),
    )
    renderPage()
    fireEvent.click(await screen.findByRole('button', { name: /show submitted content/i }))
    await waitFor(() => {
      expect(mockGET).toHaveBeenCalledWith(
        '/api/v1/mcp/servers/{namespace}/{slug}/versions/{version}',
        { params: { path: { namespace: 'acme', slug: 'weather', version: '1.0.0' } } },
      )
    })
    expect(await screen.findByText(/get_forecast, get_alerts/)).toBeInTheDocument()
    expect(screen.getByText(/npm: @acme\/weather@1\.0\.0 \(stdio\)/)).toBeInTheDocument()
  })

  it('shows a reject form on click and submits with revision + reason', async () => {
    renderPage()
    const rejectButtons = await screen.findAllByRole('button', { name: /^reject$/i })
    fireEvent.click(rejectButtons[0])

    const input = await screen.findByPlaceholderText(/reason/i)
    fireEvent.change(input, { target: { value: 'incomplete' } })
    fireEvent.click(screen.getByRole('button', { name: /confirm reject/i }))

    await waitFor(() => {
      expect(mockPOST).toHaveBeenCalledWith(
        '/api/v1/mcp/servers/{namespace}/{slug}/versions/{version}/reject',
        {
          params: { path: { namespace: 'acme', slug: 'weather', version: '1.0.0' } },
          body: { revision: 1, reason: 'incomplete' },
        },
      )
    })
  })

  it('reject submit is disabled until a reason is typed', async () => {
    renderPage()
    const rejectButtons = await screen.findAllByRole('button', { name: /^reject$/i })
    fireEvent.click(rejectButtons[0])
    const confirm = await screen.findByRole('button', { name: /confirm reject/i })
    expect(confirm).toBeDisabled()
  })

  it('surfaces a friendly error on revision-mismatch', async () => {
    mockPOST.mockResolvedValue({
      error: {
        type: 'https://registry/errors/review-revision-mismatch',
        detail: 'irrelevant',
        status: 409,
      },
    })
    renderPage()
    await approveRow(0, /^approve & publish$/i)
    expect(
      await screen.findByText(/edited since this queue page loaded/i),
    ).toBeInTheDocument()
  })
})
