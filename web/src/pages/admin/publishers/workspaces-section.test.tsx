import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor, fireEvent } from '@testing-library/react'
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
      <WorkspacesSection publisherSlug={slug} />
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
    const deleteTriggers = await screen.findAllByRole('button', { name: /^delete$/i })
    fireEvent.click(deleteTriggers[0])

    expect(
      await screen.findByText(/workspace still has mcp servers or agents/i),
    ).toBeInTheDocument()
    confirmSpy.mockRestore()
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
