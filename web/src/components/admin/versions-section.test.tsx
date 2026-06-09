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

function renderSection(kind: 'mcp' | 'agent' = 'mcp') {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter>
        <VersionsSection kind={kind} namespace="acme" slug="weather" />
      </MemoryRouter>
    </QueryClientProvider>,
  )
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

  it('Submit on a draft posts to the typed mcp submit endpoint', async () => {
    renderSection('mcp')
    const submitButtons = await screen.findAllByRole('button', { name: /^submit$/i })
    fireEvent.click(submitButtons[0])
    await waitFor(() => {
      expect(mockPOST).toHaveBeenCalledWith(
        '/api/v1/mcp/servers/{namespace}/{slug}/versions/{version}/submit',
        { params: { path: { namespace: 'acme', slug: 'weather', version: '1.0.0' } } },
      )
    })
  })

  it('Resubmit on a rejected version posts to submit (clears reason on the server)', async () => {
    renderSection('mcp')
    const resubmitButton = await screen.findByRole('button', { name: /^resubmit$/i })
    fireEvent.click(resubmitButton)
    await waitFor(() => {
      expect(mockPOST).toHaveBeenCalledWith(
        '/api/v1/mcp/servers/{namespace}/{slug}/versions/{version}/submit',
        { params: { path: { namespace: 'acme', slug: 'weather', version: '1.2.0' } } },
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
    await waitFor(() => {
      expect(mockPOST).toHaveBeenCalledWith(
        '/api/v1/agents/{namespace}/{slug}/versions/{version}/submit',
        { params: { path: { namespace: 'acme', slug: 'weather', version: '1.0.0' } } },
      )
    })
  })

  it('Publish on an unpublished version posts to the publish endpoint after confirm', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true)
    renderSection('mcp')
    // publishedV0 (0.9.0) is already published, so the first Publish button
    // belongs to the first unpublished row (draftV1, 1.0.0).
    const publishButtons = await screen.findAllByRole('button', { name: /^publish$/i })
    fireEvent.click(publishButtons[0])
    await waitFor(() => {
      expect(mockPOST).toHaveBeenCalledWith(
        '/api/v1/mcp/servers/{namespace}/{slug}/versions/{version}/publish',
        { params: { path: { namespace: 'acme', slug: 'weather', version: '1.0.0' } } },
      )
    })
    confirmSpy.mockRestore()
  })

  it('does not show Publish on an already-published version', async () => {
    renderSection('mcp')
    await screen.findByText('v0.9.0')
    // Three unpublished rows (draft, pending, rejected) → three Publish buttons,
    // never four — the published row must not offer Publish.
    const publishButtons = screen.getAllByRole('button', { name: /^publish$/i })
    expect(publishButtons).toHaveLength(3)
  })

  it('surfaces a friendly error on review-revision-mismatch', async () => {
    mockPOST.mockResolvedValue({
      error: { type: 'https://registry/errors/review-revision-mismatch' },
    })
    renderSection('mcp')
    const submitButtons = await screen.findAllByRole('button', { name: /^submit$/i })
    fireEvent.click(submitButtons[0])
    expect(
      await screen.findByText(/edited since this page loaded/i),
    ).toBeInTheDocument()
  })
})
