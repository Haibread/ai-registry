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

import AdminAgentDetail from './detail'

function renderPage() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={['/admin/agents/acme/example-agent']}>
        <Routes>
          <Route path="/admin/agents/:ns/:slug" element={<AdminAgentDetail />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

const sampleAgent = {
  id: '01HAGT1',
  namespace: 'acme',
  slug: 'example-agent',
  name: 'Example Agent',
  description: 'An example agent',
  status: 'published',
  visibility: 'public',
  created_at: '2026-04-01T10:00:00Z',
  updated_at: '2026-04-02T10:00:00Z',
  latest_version: {
    version: '0.1.0',
    endpoint_url: 'https://agent.example.test/a2a',
    protocol_version: '2025-06-18',
    published_at: '2026-04-02T10:00:00Z',
    default_input_modes: ['text'],
    default_output_modes: ['text'],
    authentication: [{ scheme: 'Bearer' }],
    skills: [
      {
        id: 'greet',
        name: 'Greeter',
        description: 'Says hi',
        tags: ['social'],
        examples: ['Hello world'],
      },
    ],
  },
}

describe('AdminAgentDetail', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockGET.mockResolvedValue({ data: sampleAgent })
    mockPOST.mockResolvedValue({})
    mockPATCH.mockResolvedValue({})
    mockDELETE.mockResolvedValue({ data: {}, error: undefined })
  })

  it('fetches the agent detail on mount', async () => {
    renderPage()
    await waitFor(() => {
      expect(mockGET).toHaveBeenCalledWith('/api/v1/agents/{namespace}/{slug}', {
        params: { path: { namespace: 'acme', slug: 'example-agent' } },
      })
    })
  })

  it('renders the heading and the agent metadata', async () => {
    renderPage()
    expect(await screen.findByRole('heading', { name: 'Example Agent' })).toBeInTheDocument()
    expect(screen.getByText('An example agent')).toBeInTheDocument()
    expect(screen.getByText('v0.1.0')).toBeInTheDocument()
    expect(screen.getByText('https://agent.example.test/a2a')).toBeInTheDocument()
  })

  it('renders skills from the latest version', async () => {
    renderPage()
    expect(await screen.findByText('Greeter')).toBeInTheDocument()
    expect(screen.getByText('Says hi')).toBeInTheDocument()
  })

  it('toggles visibility via POST when make-private is clicked', async () => {
    renderPage()
    await screen.findByRole('heading', { name: 'Example Agent' })
    fireEvent.click(screen.getByRole('button', { name: /make private/i }))
    await waitFor(() => {
      expect(mockPOST).toHaveBeenCalledWith(
        '/api/v1/agents/{namespace}/{slug}/visibility',
        {
          params: { path: { namespace: 'acme', slug: 'example-agent' } },
          body: { visibility: 'private' },
        },
      )
    })
  })

  it('submits a PATCH when the edit form is saved', async () => {
    renderPage()
    await screen.findByRole('heading', { name: 'Example Agent' })
    fireEvent.click(screen.getByRole('button', { name: /^edit$/i }))
    fireEvent.click(screen.getByRole('button', { name: /save changes/i }))
    await waitFor(() => {
      expect(mockPATCH).toHaveBeenCalledWith('/api/v1/agents/{namespace}/{slug}', {
        params: { path: { namespace: 'acme', slug: 'example-agent' } },
        body: {
          name: 'Example Agent',
          description: 'An example agent',
        },
      })
    })
  })

  it('shows a not-found state when the query errors', async () => {
    mockGET.mockRejectedValueOnce(new Error('nope'))
    renderPage()
    expect(await screen.findByText(/not found/i)).toBeInTheDocument()
  })

  // ─── Deprecate / lifecycle / delete / a2a-link coverage (v0.2.2) ─────────
  //
  // The pre-existing tests had no coverage for the DeprecateButton flow on
  // the agent side (the MCP side did), no coverage of the LifecycleStepper
  // transition, and no coverage of the A2A well-known card link that is the
  // whole point of the Agent registry per CLAUDE.md. These tests fill all of
  // those gaps plus the same delete/cancel/error trio as the MCP page.

  it('deprecates via the DeprecateButton when confirm is accepted', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true)
    renderPage()
    await screen.findByRole('heading', { name: 'Example Agent' })

    fireEvent.click(screen.getByRole('button', { name: /^deprecate$/i }))

    await waitFor(() => {
      expect(mockPOST).toHaveBeenCalledWith(
        '/api/v1/agents/{namespace}/{slug}/deprecate',
        { params: { path: { namespace: 'acme', slug: 'example-agent' } } },
      )
    })
    confirmSpy.mockRestore()
  })

  it('does not deprecate when the user declines the confirm dialog', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(false)
    renderPage()
    await screen.findByRole('heading', { name: 'Example Agent' })

    fireEvent.click(screen.getByRole('button', { name: /^deprecate$/i }))

    await Promise.resolve()
    // The visibility POST would have succeeded — we specifically assert that
    // the deprecate URL was not POSTed, which is stronger than "no POST at all".
    const deprecateCalls = mockPOST.mock.calls.filter((c) =>
      typeof c[0] === 'string' && c[0].endsWith('/deprecate'),
    )
    expect(deprecateCalls).toHaveLength(0)
    confirmSpy.mockRestore()
  })

  it('deprecates via the LifecycleStepper Deprecated transition', async () => {
    renderPage()
    await screen.findByRole('heading', { name: 'Example Agent' })

    // The stepper bypasses window.confirm — it's a one-click lifecycle
    // affordance. Query by title (the tooltip) because only the clickable
    // target carries "Transition to …" — the other stages show their label
    // or "Current status: …".
    const transitionBtn = screen.getByTitle(/transition to deprecated/i)
    fireEvent.click(transitionBtn)

    await waitFor(() => {
      expect(mockPOST).toHaveBeenCalledWith(
        '/api/v1/agents/{namespace}/{slug}/deprecate',
        { params: { path: { namespace: 'acme', slug: 'example-agent' } } },
      )
    })
  })

  it('hides the DeprecateButton when status is not published', async () => {
    mockGET.mockResolvedValueOnce({ data: { ...sampleAgent, status: 'draft' } })
    renderPage()
    await screen.findByRole('heading', { name: 'Example Agent' })

    // The DeprecateButton is only mounted when status === 'published'.
    // The LifecycleStepper still has a transition target for published→deprecated,
    // but from 'draft' the stepper's valid targets are ['published'], so no
    // "Transition to Deprecated" button exists either — only the stage label.
    expect(screen.queryByRole('button', { name: /^deprecate$/i })).not.toBeInTheDocument()
  })

  it('opens and cancels the edit form without firing a PATCH', async () => {
    renderPage()
    await screen.findByRole('heading', { name: 'Example Agent' })

    fireEvent.click(screen.getByRole('button', { name: /^edit$/i }))
    expect(screen.getByRole('heading', { name: /edit agent/i })).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: /^cancel$/i }))

    await waitFor(() =>
      expect(screen.queryByRole('heading', { name: /edit agent/i })).not.toBeInTheDocument(),
    )
    expect(mockPATCH).not.toHaveBeenCalled()
  })

  it('deletes the agent and navigates back to the list when confirmed', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true)
    renderPage()
    await screen.findByRole('heading', { name: 'Example Agent' })

    fireEvent.click(screen.getByRole('button', { name: /^delete$/i }))

    await waitFor(() => {
      expect(mockDELETE).toHaveBeenCalledWith(
        '/api/v1/agents/{namespace}/{slug}',
        { params: { path: { namespace: 'acme', slug: 'example-agent' } } },
      )
    })
    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/admin/agents')
    })
    confirmSpy.mockRestore()
  })

  it('surfaces an "Action failed" message when visibility mutation rejects', async () => {
    mockPOST.mockRejectedValueOnce(new Error('boom'))
    renderPage()
    await screen.findByRole('heading', { name: 'Example Agent' })

    fireEvent.click(screen.getByRole('button', { name: /make private/i }))

    expect(await screen.findByText(/action failed/i)).toBeInTheDocument()
  })

  it('renders the A2A agent-card well-known link with the correct href', async () => {
    renderPage()
    await screen.findByRole('heading', { name: 'Example Agent' })

    // The registry's A2A compatibility promise (CLAUDE.md Resolved Decision H)
    // hinges on this path shape — a regression here would silently break
    // every A2A client that has cached the URL.
    const link = screen.getByRole('link', { name: /view agent card/i })
    expect(link).toHaveAttribute(
      'href',
      '/agents/acme/example-agent/.well-known/agent-card.json',
    )
  })
})
