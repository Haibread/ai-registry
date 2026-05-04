import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor, fireEvent, within } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'

vi.mock('@/auth/AuthContext', () => ({
  useAuth: () => ({ accessToken: 'test-token' }),
}))

const mockGET = vi.fn()
const mockPOST = vi.fn()
const mockPATCH = vi.fn()
const mockDELETE = vi.fn()
vi.mock('@/lib/api-client', () => ({
  useAuthClient: () => ({
    GET: mockGET,
    POST: mockPOST,
    PATCH: mockPATCH,
    DELETE: mockDELETE,
  }),
}))

import { WorkspacesSection } from './workspaces-section'

function renderSection(slug = 'acme') {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter>
        <WorkspacesSection publisherSlug={slug} />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

const sampleWorkspace = {
  id: '01HWS',
  publisher_id: '01HPUB',
  slug: 'claude-team',
  name: 'Claude team',
  description: 'Team owns claude',
  group_name: 'claude-team-grp',
  created_at: '2026-04-01T00:00:00Z',
  updated_at: '2026-04-10T00:00:00Z',
}

const adminOnlyWorkspace = {
  id: '01HWS2',
  publisher_id: '01HPUB',
  slug: 'default',
  name: 'Default',
  created_at: '2026-04-01T00:00:00Z',
  updated_at: '2026-04-01T00:00:00Z',
  // group_name omitted → admin-only
}

describe('WorkspacesSection', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockGET.mockResolvedValue({ data: { items: [sampleWorkspace, adminOnlyWorkspace] } })
    mockPOST.mockResolvedValue({})
    mockPATCH.mockResolvedValue({})
    mockDELETE.mockResolvedValue({})
  })

  it('renders the workspaces heading and count', async () => {
    renderSection()
    expect(await screen.findByRole('heading', { name: /workspaces/i })).toBeInTheDocument()
    // The count renders inside a sibling span and only after the GET
    // resolves; findByText waits for the post-fetch render.
    expect(await screen.findByText('(2)')).toBeInTheDocument()
  })

  it('lists each workspace with its group_name or admin-only fallback', async () => {
    renderSection()
    expect(await screen.findByText('claude-team')).toBeInTheDocument()
    expect(screen.getByText('default')).toBeInTheDocument()
    expect(screen.getByText('claude-team-grp')).toBeInTheDocument()
    expect(screen.getByText('admin-only')).toBeInTheDocument()
  })

  it('renders the empty-state copy when there are no workspaces', async () => {
    mockGET.mockResolvedValue({ data: { items: [] } })
    renderSection()
    expect(
      await screen.findByText(/no workspaces yet/i),
    ).toBeInTheDocument()
  })

  it('opens the create form and POSTs the workspace on submit', async () => {
    renderSection()
    fireEvent.click(await screen.findByRole('button', { name: /new workspace/i }))

    fireEvent.change(screen.getByLabelText(/^slug/i), { target: { value: 'new-team' } })
    fireEvent.change(screen.getByLabelText(/^name/i), { target: { value: 'New team' } })
    fireEvent.change(screen.getByLabelText(/^description$/i), {
      target: { value: 'desc' },
    })
    fireEvent.click(screen.getByRole('button', { name: /create workspace/i }))

    await waitFor(() => {
      expect(mockPOST).toHaveBeenCalledWith(
        '/api/v1/publishers/{publisher_slug}/workspaces',
        {
          params: { path: { publisher_slug: 'acme' } },
          body: { slug: 'new-team', name: 'New team', description: 'desc' },
        },
      )
    })
  })

  it('opens an edit form scoped to a row and PATCHes group_name', async () => {
    renderSection()
    const editButtons = await screen.findAllByRole('button', { name: /^edit$/i })
    fireEvent.click(editButtons[0]) // first row → claude-team

    const groupInput = await screen.findByLabelText(/keycloak group/i)
    fireEvent.change(groupInput, { target: { value: 'new-group' } })
    fireEvent.click(screen.getByRole('button', { name: /save changes/i }))

    await waitFor(() => {
      expect(mockPATCH).toHaveBeenCalledWith(
        '/api/v1/publishers/{publisher_slug}/workspaces/{workspace_slug}',
        {
          params: { path: { publisher_slug: 'acme', workspace_slug: 'claude-team' } },
          body: expect.objectContaining({ group_name: 'new-group' }),
        },
      )
    })
  })

  it('opens the edit form inside a modal dialog rather than inline', async () => {
    renderSection()
    const editButtons = await screen.findAllByRole('button', { name: /^edit$/i })
    fireEvent.click(editButtons[0])

    // The form is wrapped in a role="dialog" with aria-modal so screen
    // readers treat it as a modal and the testing-library role queries
    // can locate the surrounding container, not just the form fields.
    const dialog = await screen.findByRole('dialog', { name: /edit workspace/i })
    expect(dialog).toHaveAttribute('aria-modal', 'true')

    // The dialog has a Close (X) button in its header in addition to
    // the form's Cancel button. Clicking Close dismisses the dialog
    // without firing a PATCH.
    fireEvent.click(within(dialog).getByRole('button', { name: /^close$/i }))
    await waitFor(() => {
      expect(screen.queryByRole('dialog', { name: /edit workspace/i })).toBeNull()
    })
    expect(mockPATCH).not.toHaveBeenCalled()
  })

  it('clearing the group_name in edit submits an empty string (admin-only)', async () => {
    renderSection()
    const editButtons = await screen.findAllByRole('button', { name: /^edit$/i })
    fireEvent.click(editButtons[0])

    const groupInput = await screen.findByLabelText(/keycloak group/i)
    fireEvent.change(groupInput, { target: { value: '' } })
    fireEvent.click(screen.getByRole('button', { name: /save changes/i }))

    await waitFor(() => {
      expect(mockPATCH).toHaveBeenCalledWith(
        '/api/v1/publishers/{publisher_slug}/workspaces/{workspace_slug}',
        expect.objectContaining({
          body: expect.objectContaining({ group_name: '' }),
        }),
      )
    })
  })

  it('surfaces a friendly error when delete fails because the workspace is non-empty', async () => {
    // DeleteButton uses window.confirm — auto-accept it.
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true)
    mockDELETE.mockResolvedValue({
      error: { status: 409, detail: 'workspace still has resources' },
    })
    renderSection()
    const deleteTriggers = await screen.findAllByRole('button', { name: /delete workspace/i })
    fireEvent.click(deleteTriggers[0])

    expect(
      await screen.findByText(/workspace still has mcp servers or agents/i),
    ).toBeInTheDocument()
    confirmSpy.mockRestore()
  })

  describe('row expansion', () => {
    it('hides the inline contents until the chevron toggle is clicked', async () => {
      // Tailor the GET mock to differentiate the workspaces-list call from
      // the expansion calls so the test asserts on URLs explicitly.
      mockGET.mockImplementation((url: string) => {
        if (url === '/api/v1/publishers/{publisher_slug}/workspaces') {
          return Promise.resolve({ data: { items: [sampleWorkspace] } })
        }
        if (url.endsWith('/servers')) {
          return Promise.resolve({
            data: { items: [{ id: 's1', name: 'My MCP', slug: 'my-mcp', namespace: 'acme', status: 'published', updated_at: '2026-04-12' }] },
          })
        }
        if (url.endsWith('/agents')) {
          return Promise.resolve({ data: { items: [] } })
        }
        return Promise.resolve({ data: { items: [] } })
      })

      renderSection()
      // Wait for the workspace row.
      await screen.findByText('claude-team')

      // Initially collapsed: the MCP entry must not be in the DOM.
      expect(screen.queryByText('My MCP')).toBeNull()

      const toggle = screen.getByRole('button', { name: /expand workspace contents/i })
      expect(toggle).toHaveAttribute('aria-expanded', 'false')
      fireEvent.click(toggle)

      // After clicking, the GET for /servers and /agents fire and the
      // panel renders.
      expect(await screen.findByText('My MCP')).toBeInTheDocument()
      expect(screen.getByText(/no agents in this workspace/i)).toBeInTheDocument()

      // aria-expanded flips and the toggle label updates so screen-reader
      // users can collapse it again.
      const collapseToggle = screen.getByRole('button', { name: /collapse workspace contents/i })
      expect(collapseToggle).toHaveAttribute('aria-expanded', 'true')
    })

    it('renders Manage links pointing to the entity admin page', async () => {
      mockGET.mockImplementation((url: string) => {
        if (url === '/api/v1/publishers/{publisher_slug}/workspaces') {
          return Promise.resolve({ data: { items: [sampleWorkspace] } })
        }
        if (url.endsWith('/servers')) {
          return Promise.resolve({
            data: { items: [{ id: 's1', name: 'My MCP', slug: 'my-mcp', namespace: 'acme', status: 'draft', updated_at: '2026-04-12' }] },
          })
        }
        if (url.endsWith('/agents')) {
          return Promise.resolve({
            data: { items: [{ id: 'a1', name: 'My Agent', slug: 'my-agent', namespace: 'acme', status: 'published', updated_at: '2026-04-12' }] },
          })
        }
        return Promise.resolve({ data: { items: [] } })
      })

      renderSection()
      fireEvent.click(await screen.findByRole('button', { name: /expand workspace contents/i }))

      const allManage = await screen.findAllByRole('link', { name: /manage/i })
      // The MCP list renders first, so the first Manage points at the
      // server admin page and the second at the agent admin page.
      expect(allManage[0]).toHaveAttribute('href', '/admin/mcp/acme/my-mcp')
      expect(allManage[1]).toHaveAttribute('href', '/admin/agents/acme/my-agent')
    })

    it('surfaces a load error if either resource list fails', async () => {
      mockGET.mockImplementation((url: string) => {
        if (url === '/api/v1/publishers/{publisher_slug}/workspaces') {
          return Promise.resolve({ data: { items: [sampleWorkspace] } })
        }
        if (url.endsWith('/servers')) {
          return Promise.reject(new Error('boom'))
        }
        if (url.endsWith('/agents')) {
          return Promise.resolve({ data: { items: [] } })
        }
        return Promise.resolve({ data: { items: [] } })
      })

      renderSection()
      fireEvent.click(await screen.findByRole('button', { name: /expand workspace contents/i }))

      expect(
        await screen.findByText(/failed to load workspace contents/i),
      ).toBeInTheDocument()
    })
  })

  it('surfaces a friendly error when create returns conflict on duplicate slug', async () => {
    // Server omits `detail` here so the friendly fallback message kicks
    // in. When `detail` is provided, friendlyError prefers the server's
    // wording.
    mockPOST.mockResolvedValue({
      error: { type: 'https://registry/errors/conflict' },
    })
    renderSection()
    fireEvent.click(await screen.findByRole('button', { name: /new workspace/i }))
    fireEvent.change(screen.getByLabelText(/^slug/i), { target: { value: 'dup' } })
    fireEvent.change(screen.getByLabelText(/^name/i), { target: { value: 'Dup' } })
    fireEvent.click(screen.getByRole('button', { name: /create workspace/i }))

    expect(
      await screen.findByText(/slug already exists in this publisher/i),
    ).toBeInTheDocument()
  })
})
