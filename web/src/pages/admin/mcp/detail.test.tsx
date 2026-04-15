import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor, fireEvent } from '@testing-library/react'
import { MemoryRouter, Routes, Route } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'

vi.mock('@/auth/AuthContext', () => ({
  useAuth: () => ({ accessToken: 'test-token' }),
}))

const mockNavigate = vi.fn()
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom')
  return { ...actual, useNavigate: () => mockNavigate }
})

const mockGET = vi.fn()
const mockPOST = vi.fn()
const mockPATCH = vi.fn()
const mockDELETE = vi.fn()
vi.mock('@/lib/api-client', () => ({
  useAuthClient: () => ({ GET: mockGET, POST: mockPOST, PATCH: mockPATCH, DELETE: mockDELETE }),
}))

import AdminMCPDetail from './detail'

function renderPage() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={['/admin/mcp/acme/example-mcp']}>
        <Routes>
          <Route path="/admin/mcp/:ns/:slug" element={<AdminMCPDetail />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

const sampleServer = {
  id: '01HMCP1',
  namespace: 'acme',
  slug: 'example-mcp',
  name: 'Example MCP',
  description: 'An example MCP server',
  status: 'published',
  visibility: 'public',
  license: 'MIT',
  homepage_url: 'https://example.test',
  repo_url: 'https://github.com/acme/example',
  created_at: '2026-04-01T10:00:00Z',
  updated_at: '2026-04-02T10:00:00Z',
  latest_version: {
    version: '1.2.3',
    // `runtime` = MCP transport mechanism (see server/internal/domain/mcp.go).
    runtime: 'http',
    protocol_version: '2025-06-18',
    published_at: '2026-04-02T10:00:00Z',
    packages: [],
  },
}

describe('AdminMCPDetail', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockGET.mockResolvedValue({ data: sampleServer })
    mockPOST.mockResolvedValue({})
    mockPATCH.mockResolvedValue({})
    mockDELETE.mockResolvedValue({ data: {}, error: undefined })
  })

  it('fetches the server detail on mount', async () => {
    renderPage()
    await waitFor(() => {
      expect(mockGET).toHaveBeenCalledWith('/api/v1/mcp/servers/{namespace}/{slug}', {
        params: { path: { namespace: 'acme', slug: 'example-mcp' } },
      })
    })
  })

  it('renders the heading and key metadata fields', async () => {
    renderPage()
    expect(await screen.findByRole('heading', { name: 'Example MCP' })).toBeInTheDocument()
    expect(screen.getByText('An example MCP server')).toBeInTheDocument()
    expect(screen.getByText('v1.2.3')).toBeInTheDocument()
    expect(screen.getByText('MIT')).toBeInTheDocument()
  })

  it('toggles visibility via POST when the make-private button is clicked', async () => {
    renderPage()
    await screen.findByRole('heading', { name: 'Example MCP' })
    fireEvent.click(screen.getByRole('button', { name: /make private/i }))
    await waitFor(() => {
      expect(mockPOST).toHaveBeenCalledWith(
        '/api/v1/mcp/servers/{namespace}/{slug}/visibility',
        {
          params: { path: { namespace: 'acme', slug: 'example-mcp' } },
          body: { visibility: 'private' },
        },
      )
    })
  })

  it('deprecates the server when the confirm dialog is accepted', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true)
    renderPage()
    await screen.findByRole('heading', { name: 'Example MCP' })
    fireEvent.click(screen.getByRole('button', { name: /^deprecate$/i }))
    await waitFor(() => {
      expect(mockPOST).toHaveBeenCalledWith(
        '/api/v1/mcp/servers/{namespace}/{slug}/deprecate',
        { params: { path: { namespace: 'acme', slug: 'example-mcp' } } },
      )
    })
    confirmSpy.mockRestore()
  })

  it('submits a PATCH when the edit form is saved', async () => {
    renderPage()
    await screen.findByRole('heading', { name: 'Example MCP' })
    fireEvent.click(screen.getByRole('button', { name: /^edit$/i }))
    fireEvent.click(screen.getByRole('button', { name: /save changes/i }))
    await waitFor(() => {
      expect(mockPATCH).toHaveBeenCalledWith('/api/v1/mcp/servers/{namespace}/{slug}', {
        params: { path: { namespace: 'acme', slug: 'example-mcp' } },
        body: {
          name: 'Example MCP',
          description: 'An example MCP server',
          homepage_url: 'https://example.test',
          repo_url: 'https://github.com/acme/example',
          license: 'MIT',
        },
      })
    })
  })

  it('shows a not-found state when the query errors', async () => {
    mockGET.mockRejectedValueOnce(new Error('nope'))
    renderPage()
    expect(await screen.findByText(/not found/i)).toBeInTheDocument()
  })

  // ─── Lifecycle / delete / error-surfacing coverage (v0.2.2) ───────────────
  //
  // The previous batch covered the happy-path flows (visibility, deprecate,
  // PATCH edit). What was missing — and is what makes the admin page actually
  // trustworthy — is the LifecycleStepper transition (which lives in a
  // separate component and is wired via a render-prop callback), the edit
  // cancel flow (state must reset without firing a mutation), the full
  // delete-confirm → DELETE → navigate chain, and the failure path where a
  // mutation errors out and the UI surfaces a retry hint.

  it('deprecates via the LifecycleStepper Deprecated transition', async () => {
    renderPage()
    await screen.findByRole('heading', { name: 'Example MCP' })

    // The LifecycleStepper renders a clickable button for each target state.
    // `defaultAllowedTransitions('published')` returns ['deprecated'], so the
    // Deprecated stage should be clickable and fire the same POST that the
    // DeprecateButton does — but WITHOUT going through window.confirm.
    // The button's text content is "Deprecated" and only the clickable
    // target has `title="Transition to …"`; querying by title keeps this
    // unambiguous without reaching into classnames.
    const transitionBtn = screen.getByTitle(/transition to deprecated/i)
    fireEvent.click(transitionBtn)

    await waitFor(() => {
      expect(mockPOST).toHaveBeenCalledWith(
        '/api/v1/mcp/servers/{namespace}/{slug}/deprecate',
        { params: { path: { namespace: 'acme', slug: 'example-mcp' } } },
      )
    })
  })

  it('opens and cancels the edit form without firing a PATCH', async () => {
    renderPage()
    await screen.findByRole('heading', { name: 'Example MCP' })

    fireEvent.click(screen.getByRole('button', { name: /^edit$/i }))
    // The edit form's own heading anchors the assertion — it is *inside* the
    // form, not the top-level page heading.
    expect(screen.getByRole('heading', { name: /edit mcp server/i })).toBeInTheDocument()

    // There are now TWO "cancel"-shaped buttons once the form is open: the
    // in-form Cancel, and the top-level "Cancel edit" toggle. Click the form's
    // Cancel — that's the one the user actually reaches for.
    fireEvent.click(screen.getByRole('button', { name: /^cancel$/i }))

    await waitFor(() =>
      expect(screen.queryByRole('heading', { name: /edit mcp server/i })).not.toBeInTheDocument(),
    )
    expect(mockPATCH).not.toHaveBeenCalled()
  })

  it('deletes the server and navigates back to the list when confirmed', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true)
    renderPage()
    await screen.findByRole('heading', { name: 'Example MCP' })

    fireEvent.click(screen.getByRole('button', { name: /^delete$/i }))

    await waitFor(() => {
      expect(mockDELETE).toHaveBeenCalledWith(
        '/api/v1/mcp/servers/{namespace}/{slug}',
        { params: { path: { namespace: 'acme', slug: 'example-mcp' } } },
      )
    })
    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/admin/mcp')
    })
    confirmSpy.mockRestore()
  })

  it('does not delete when the user declines the confirm dialog', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(false)
    renderPage()
    await screen.findByRole('heading', { name: 'Example MCP' })

    fireEvent.click(screen.getByRole('button', { name: /^delete$/i }))

    // Nothing to wait for — the user said no, so the call should never happen.
    // Give React Query a microtask to settle, then assert.
    await Promise.resolve()
    expect(mockDELETE).not.toHaveBeenCalled()
    expect(mockNavigate).not.toHaveBeenCalled()
    confirmSpy.mockRestore()
  })

  it('surfaces an "Action failed" message when visibility mutation rejects', async () => {
    mockPOST.mockRejectedValueOnce(new Error('boom'))
    renderPage()
    await screen.findByRole('heading', { name: 'Example MCP' })

    fireEvent.click(screen.getByRole('button', { name: /make private/i }))

    expect(await screen.findByText(/action failed/i)).toBeInTheDocument()
  })
})
