import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor, fireEvent, within } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'

// JSDOM doesn't implement HTMLDialogElement's modal methods; stub them so the
// publish confirmation (ConfirmDialog) opens and closes.
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

vi.mock('@/auth/AuthContext', () => ({
  useAuth: () => ({ accessToken: 'test-token' }),
}))

const mockGET = vi.fn()
const mockPOST = vi.fn()
vi.mock('@/lib/api-client', () => ({
  useAuthClient: () => ({ GET: mockGET, POST: mockPOST }),
}))

// Default the caller to a Server Admin so the submit/withdraw actions render
// (they are gated on canEdit for the resource's publisher).
vi.mock('@/auth/useMe', () => ({
  usePermissions: () => ({
    isLoading: false,
    isAuthenticated: true,
    isServerAdmin: true,
    grants: [],
    rolesOn: () => new Set(),
    canEdit: () => true,
    canReview: () => true,
    canAdmin: () => true,
    isEditorAnywhere: true,
    isReviewerAnywhere: true,
  }),
  useMe: () => ({ data: { authenticated: true, is_server_admin: true, grants: [] }, isLoading: false }),
  satisfiesRole: () => true,
}))

import { VersionsSection } from './versions-section'

function renderSection(kind: 'mcp' | 'agent' = 'mcp', entryVisibility: 'public' | 'private' = 'private') {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter>
        <VersionsSection kind={kind} namespace="acme" slug="weather" entryVisibility={entryVisibility} />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

// Click through the submit confirmation (submit is no longer one bare click —
// the dialog hosts the author's public-release request).
function confirmSubmitDialog() {
  const dialog = document.querySelector('dialog')!
  fireEvent.click(within(dialog).getByRole('button', { name: /^submit for review$/i }))
}

const draftV1 = {
  id: '01HV1',
  version: '1.0.0',
  runtime: 'stdio',
  protocol_version: '2024-11-05',
  status: 'active',
  created_at: '2026-04-01T00:00:00Z',
  updated_at: '2026-04-01T00:00:00Z',
  // review_state defaults to "none" / undefined; published_at unset → draft.
  revision: 1,
}

const pendingV2 = {
  ...draftV1,
  id: '01HV2',
  version: '1.1.0',
  review_state: 'pending_review',
  submitted_at: '2026-04-05T00:00:00Z',
  submitted_by_email: 'alice@example.com',
  revision: 2,
}

const rejectedV3 = {
  ...draftV1,
  id: '01HV3',
  version: '1.2.0',
  review_state: 'rejected',
  submitted_at: '2026-04-06T00:00:00Z',
  submitted_by_email: 'alice@example.com',
  reviewed_at: '2026-04-07T00:00:00Z',
  reviewed_by_email: 'bob@example.com',
  review_decision: 'rejected',
  rejection_reason: 'missing docs',
  revision: 3,
}

const publishedV0 = {
  ...draftV1,
  id: '01HV0',
  version: '0.9.0',
  status: 'active',
  published_at: '2026-03-15T00:00:00Z',
  reviewed_at: '2026-03-15T00:00:00Z',
  reviewed_by_email: 'bob@example.com',
  review_decision: 'approved',
}

describe('VersionsSection', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockGET.mockResolvedValue({
      data: { items: [publishedV0, draftV1, pendingV2, rejectedV3] },
    })
    mockPOST.mockResolvedValue({})
  })

  it('renders heading with version count', async () => {
    renderSection()
    expect(await screen.findByRole('heading', { name: /versions/i })).toBeInTheDocument()
    expect(await screen.findByText('(4)')).toBeInTheDocument()
  })

  it('renders state badges per row (published / draft / pending / rejected)', async () => {
    renderSection()
    expect(await screen.findByText('v0.9.0')).toBeInTheDocument()
    expect(screen.getByText('published')).toBeInTheDocument()
    expect(screen.getAllByText('draft').length).toBeGreaterThan(0)
    expect(screen.getByText('pending review')).toBeInTheDocument()
    expect(screen.getByText('rejected')).toBeInTheDocument()
  })

  it('shows the rejection reason inline', async () => {
    renderSection()
    expect(await screen.findByText(/missing docs/i)).toBeInTheDocument()
  })

  it('Submit on a draft confirms first, then posts to the typed mcp submit endpoint', async () => {
    renderSection('mcp')
    const submitButtons = await screen.findAllByRole('button', { name: /^submit$/i })
    fireEvent.click(submitButtons[0])
    // One click must not submit — the dialog names the entry + version.
    expect(mockPOST).not.toHaveBeenCalled()
    expect(screen.getByRole('heading', { name: /submit acme\/weather v1\.0\.0 for review\?/i })).toBeInTheDocument()
    confirmSubmitDialog()
    await waitFor(() => {
      expect(mockPOST).toHaveBeenCalledWith(
        '/api/v1/mcp/servers/{namespace}/{slug}/versions/{version}/submit',
        { params: { path: { namespace: 'acme', slug: 'weather', version: '1.0.0' } }, body: undefined },
      )
    })
  })

  it('the submit dialog carries the public-release request on a private entry', async () => {
    renderSection('mcp', 'private')
    const submitButtons = await screen.findAllByRole('button', { name: /^submit$/i })
    fireEvent.click(submitButtons[0])
    const checkbox = screen.getByLabelText(/also request making this entry public/i)
    fireEvent.click(checkbox)
    confirmSubmitDialog()
    await waitFor(() => {
      expect(mockPOST).toHaveBeenCalledWith(
        '/api/v1/mcp/servers/{namespace}/{slug}/versions/{version}/submit',
        {
          params: { path: { namespace: 'acme', slug: 'weather', version: '1.0.0' } },
          body: { request_public: true },
        },
      )
    })
  })

  it('offers no public-release request on an already-public entry', async () => {
    renderSection('mcp', 'public')
    const submitButtons = await screen.findAllByRole('button', { name: /^submit$/i })
    fireEvent.click(submitButtons[0])
    expect(screen.getByRole('heading', { name: /submit acme\/weather v1\.0\.0 for review\?/i })).toBeInTheDocument()
    expect(screen.queryByLabelText(/also request making this entry public/i)).not.toBeInTheDocument()
  })

  it('flags a pending submission that requested a public release', async () => {
    mockGET.mockResolvedValue({
      data: { items: [{ ...pendingV2, request_public: true }] },
    })
    renderSection('mcp')
    expect(await screen.findByText(/public on approval/i)).toBeInTheDocument()
  })

  it('Resubmit on a rejected version posts to submit (clears reason on the server)', async () => {
    renderSection('mcp')
    const resubmitButton = await screen.findByRole('button', { name: /^resubmit$/i })
    fireEvent.click(resubmitButton)
    confirmSubmitDialog()
    await waitFor(() => {
      expect(mockPOST).toHaveBeenCalledWith(
        '/api/v1/mcp/servers/{namespace}/{slug}/versions/{version}/submit',
        { params: { path: { namespace: 'acme', slug: 'weather', version: '1.2.0' } }, body: undefined },
      )
    })
  })

  it('Withdraw on a pending version posts to the withdraw endpoint', async () => {
    renderSection('mcp')
    const withdrawButton = await screen.findByRole('button', { name: /^withdraw$/i })
    fireEvent.click(withdrawButton)
    await waitFor(() => {
      expect(mockPOST).toHaveBeenCalledWith(
        '/api/v1/mcp/servers/{namespace}/{slug}/versions/{version}/withdraw',
        { params: { path: { namespace: 'acme', slug: 'weather', version: '1.1.0' } } },
      )
    })
  })

  it('Submit on the agent kind targets the agent endpoint family', async () => {
    renderSection('agent')
    const submitButtons = await screen.findAllByRole('button', { name: /^submit$/i })
    fireEvent.click(submitButtons[0])
    confirmSubmitDialog()
    await waitFor(() => {
      expect(mockPOST).toHaveBeenCalledWith(
        '/api/v1/agents/{namespace}/{slug}/versions/{version}/submit',
        { params: { path: { namespace: 'acme', slug: 'weather', version: '1.0.0' } }, body: undefined },
      )
    })
  })

  it('Publish on a draft posts to the publish endpoint after the confirm dialog', async () => {
    renderSection('mcp')
    // publishedV0 (0.9.0) is already published, so the first Publish button
    // belongs to the first unpublished row (draftV1, 1.0.0).
    const publishButtons = await screen.findAllByRole('button', { name: /^publish$/i })
    fireEvent.click(publishButtons[0])
    // Nothing fires until the themed dialog (naming entry + version) confirms.
    expect(mockPOST).not.toHaveBeenCalled()
    expect(screen.getByRole('heading', { name: /publish acme\/weather v1\.0\.0\?/i })).toBeInTheDocument()
    const dialog = document.querySelector('dialog')!
    fireEvent.click(within(dialog).getByRole('button', { name: /^publish$/i }))
    await waitFor(() => {
      expect(mockPOST).toHaveBeenCalledWith(
        '/api/v1/mcp/servers/{namespace}/{slug}/versions/{version}/publish',
        { params: { path: { namespace: 'acme', slug: 'weather', version: '1.0.0' } } },
      )
    })
  })

  it('uses the queue\'s "Approve & publish" verb on a pending-review row', async () => {
    renderSection('mcp')
    // pendingV2 (1.1.0) is the only pending_review row.
    const approveBtn = await screen.findByRole('button', { name: /^approve & publish$/i })
    fireEvent.click(approveBtn)
    expect(screen.getByRole('heading', { name: /approve acme\/weather v1\.1\.0\?/i })).toBeInTheDocument()
    const dialog = document.querySelector('dialog')!
    fireEvent.click(within(dialog).getByRole('button', { name: /^approve & publish$/i }))
    await waitFor(() => {
      expect(mockPOST).toHaveBeenCalledWith(
        '/api/v1/mcp/servers/{namespace}/{slug}/versions/{version}/publish',
        { params: { path: { namespace: 'acme', slug: 'weather', version: '1.1.0' } } },
      )
    })
  })

  it('does not show Publish on an already-published version', async () => {
    renderSection('mcp')
    await screen.findByText('v0.9.0')
    // Three unpublished rows: draft + rejected say "Publish", the pending row
    // says "Approve & publish" — the published row offers neither.
    expect(screen.getAllByRole('button', { name: /^publish$/i })).toHaveLength(2)
    expect(screen.getAllByRole('button', { name: /^approve & publish$/i })).toHaveLength(1)
  })

  it('surfaces a friendly error on review-revision-mismatch', async () => {
    mockPOST.mockResolvedValue({
      error: { type: 'https://registry/errors/review-revision-mismatch' },
    })
    renderSection('mcp')
    const submitButtons = await screen.findAllByRole('button', { name: /^submit$/i })
    fireEvent.click(submitButtons[0])
    confirmSubmitDialog()
    expect(
      await screen.findByText(/edited since this page loaded/i),
    ).toBeInTheDocument()
  })
})
