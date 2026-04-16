import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor, fireEvent, within } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'

vi.mock('@/auth/AuthContext', () => ({
  useAuth: () => ({ accessToken: 'test-token' }),
}))

const mockGET = vi.fn()
vi.mock('@/lib/api-client', () => ({
  useAuthClient: () => ({ GET: mockGET }),
}))

import AdminAudit from './audit'

function renderPage() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter>
        <AdminAudit />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

const events = [
  {
    id: '01HX_CREATE',
    actor_subject: 'kc-uuid-a',
    actor_email: 'alice@example.com',
    action: 'mcp_server.created',
    resource_type: 'mcp_server',
    resource_id: 'srv-01',
    resource_ns: 'acme',
    resource_slug: 'cool-server',
    metadata: { source: 'bootstrap' },
    created_at: '2026-04-15T10:00:00Z',
  },
  {
    id: '01HX_PUB',
    actor_subject: 'kc-uuid-a',
    actor_email: 'alice@example.com',
    action: 'mcp_server_version.published',
    resource_type: 'mcp_server',
    resource_id: 'srv-01',
    resource_ns: 'acme',
    resource_slug: 'cool-server',
    metadata: { version: '1.0.0' },
    created_at: '2026-04-15T11:30:00Z',
  },
  {
    id: '01HX_DEP',
    actor_subject: 'kc-uuid-b',
    actor_email: 'bob@example.com',
    action: 'agent.deprecated',
    resource_type: 'agent',
    resource_id: 'ag-01',
    resource_ns: 'acme',
    resource_slug: 'planner',
    metadata: { reason: 'obsolete' },
    created_at: '2026-04-15T12:45:00Z',
  },
]

describe('AdminAudit page', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockGET.mockResolvedValue({ data: { items: events, next_cursor: '' } })
  })

  it('renders the page heading', async () => {
    renderPage()
    expect(screen.getByRole('heading', { name: /activity/i })).toBeInTheDocument()
  })

  it('fetches /api/v1/audit on mount with default limit', async () => {
    renderPage()
    await waitFor(() => {
      expect(mockGET).toHaveBeenCalledWith(
        '/api/v1/audit',
        expect.objectContaining({
          params: expect.objectContaining({
            query: expect.objectContaining({ limit: '50' }),
          }),
        }),
      )
    })
  })

  it('renders one row per event from the API', async () => {
    renderPage()
    await screen.findByText('mcp_server.created')
    expect(screen.getByText('mcp_server_version.published')).toBeInTheDocument()
    expect(screen.getByText('agent.deprecated')).toBeInTheDocument()
  })

  it('shows actor identity (email + subject) on every row', async () => {
    renderPage()
    await screen.findByText('mcp_server.created')
    expect(screen.getAllByText('alice@example.com').length).toBeGreaterThanOrEqual(1)
    expect(screen.getByText('bob@example.com')).toBeInTheDocument()
    expect(screen.getAllByText('kc-uuid-a').length).toBeGreaterThanOrEqual(1)
  })

  it('renders drill-down links to the admin detail pages', async () => {
    renderPage()
    await screen.findByText('mcp_server.created')
    const mcpLinks = screen.getAllByRole('link', { name: /acme\/cool-server/i })
    expect(mcpLinks[0]).toHaveAttribute('href', '/admin/mcp/acme/cool-server')
    const agentLink = screen.getByRole('link', { name: /acme\/planner/i })
    expect(agentLink).toHaveAttribute('href', '/admin/agents/acme/planner')
  })

  it('filters by resource_type when the select is changed', async () => {
    renderPage()
    await screen.findByText('mcp_server.created')
    mockGET.mockClear()
    // Radix Select uses role="combobox"; click the trigger then the option.
    const typeTrigger = screen.getByLabelText('Resource type')
    fireEvent.click(typeTrigger)
    const agentOption = await screen.findByRole('option', { name: /agents/i })
    fireEvent.click(agentOption)
    await waitFor(() => {
      const call = mockGET.mock.calls.find(
        (c) => c[1]?.params?.query?.resource_type === 'agent',
      )
      expect(call).toBeDefined()
    })
  })

  it('sends the actor filter when Apply is clicked', async () => {
    renderPage()
    await screen.findByText('mcp_server.created')
    mockGET.mockClear()
    fireEvent.change(screen.getByLabelText('Actor subject'), {
      target: { value: 'kc-uuid-b' },
    })
    fireEvent.click(screen.getByRole('button', { name: /apply/i }))
    await waitFor(() => {
      const call = mockGET.mock.calls.find(
        (c) => c[1]?.params?.query?.actor === 'kc-uuid-b',
      )
      expect(call).toBeDefined()
    })
  })

  it('expands a row to show raw metadata on click', async () => {
    renderPage()
    await screen.findByText('mcp_server.created')
    // First expand button on the first row
    const expandBtn = screen.getAllByRole('button', { name: /expand row/i })[0]
    fireEvent.click(expandBtn)
    // Metadata is now rendered as JSON pretty-print
    expect(await screen.findByText(/"source": "bootstrap"/)).toBeInTheDocument()
  })

  it('client-side filters by action', async () => {
    renderPage()
    await screen.findByText('mcp_server.created')
    const actionTrigger = screen.getByLabelText('Action')
    fireEvent.click(actionTrigger)
    const opt = await screen.findByRole('option', { name: 'agent.deprecated' })
    fireEvent.click(opt)
    // Only the agent.deprecated row should remain visible — scope the
    // assertion to the rows list so the select trigger (which now also
    // displays "agent.deprecated") doesn't muddy the match.
    const rows = await screen.findByTestId('audit-rows')
    expect(within(rows).queryByText('mcp_server.created')).not.toBeInTheDocument()
    expect(within(rows).getByText('agent.deprecated')).toBeInTheDocument()
  })

  it('shows empty state when API returns zero events', async () => {
    mockGET.mockResolvedValue({ data: { items: [], next_cursor: '' } })
    renderPage()
    expect(
      await screen.findByText(/no audit events match/i),
    ).toBeInTheDocument()
  })

  it('shows Load more when next_cursor is present', async () => {
    mockGET.mockResolvedValue({
      data: { items: events, next_cursor: 'NEXT-PAGE-CURSOR' },
    })
    renderPage()
    expect(
      await screen.findByRole('button', { name: /load more/i }),
    ).toBeInTheDocument()
  })
})
