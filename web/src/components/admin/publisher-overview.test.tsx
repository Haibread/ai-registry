import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import type { PublisherOption } from '@/auth/PublisherContext'

vi.mock('@/auth/AuthContext', () => ({ useAuth: () => ({ accessToken: 't' }) }))

const mockGET = vi.fn()
vi.mock('@/lib/api-client', () => ({ useAuthClient: () => ({ GET: mockGET }) }))

interface MockPerms {
  canEdit: (slug: string) => boolean
  canReview: (slug: string) => boolean
}
let mockPerms: MockPerms
vi.mock('@/auth/useMe', () => ({ usePermissions: () => mockPerms }))

import { PublisherOverview } from './publisher-overview'

const stats = {
  mcp_servers: 12,
  agents: 5,
  members: 8,
  pending_review: 3,
  mcp_status_breakdown: { draft: 2, published: 9, deprecated: 1 },
  agent_status_breakdown: { draft: 1, published: 4, deprecated: 0 },
  member_roles: { viewers: 3, editors: 3, reviewers: 2, admins: 0 },
}

const activity = {
  items: [
    {
      id: '1',
      action: 'mcp_server_version.published',
      resource_type: 'mcp_server',
      resource_slug: 'weather',
      version: '1.4.0',
      actor_email: 'dana@x.test',
      created_at: new Date().toISOString(),
    },
  ],
  next_cursor: '',
}

const option: PublisherOption = { slug: 'acme', name: 'Acme', roles: ['editor', 'reviewer'] }

function renderOverview() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter>
        <PublisherOverview slug="acme" option={option} />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

beforeEach(() => {
  vi.clearAllMocks()
  mockPerms = { canEdit: () => true, canReview: () => true }
  mockGET.mockImplementation((path: string) => {
    if (path === '/api/v1/publishers/{slug}/stats') return Promise.resolve({ data: stats })
    if (path === '/api/v1/publishers/{slug}/activity') return Promise.resolve({ data: activity })
    return Promise.resolve({ data: {} })
  })
})

describe('PublisherOverview', () => {
  it('shows scoped metrics, role chips, and the activity timeline', async () => {
    renderOverview()
    expect(await screen.findByText('12')).toBeInTheDocument() // MCP servers
    expect(screen.getByText('5')).toBeInTheDocument() // agents
    expect(screen.queryByText('Members')).not.toBeInTheDocument()
    expect(screen.getByText('2 draft · 9 published · 1 deprecated')).toBeInTheDocument() // MCP status caption
    expect(screen.getByText('editor')).toBeInTheDocument()
    expect(screen.getByText('reviewer')).toBeInTheDocument()
    expect(await screen.findByText('weather@1.4.0')).toBeInTheDocument()
  })

  it('offers create actions for both resource types behind the New menu (Editor)', async () => {
    renderOverview()
    await userEvent.click(await screen.findByRole('button', { name: /^new/i }))
    expect(await screen.findByRole('menuitem', { name: /new mcp server/i })).toHaveAttribute(
      'href',
      '/admin/mcp/new',
    )
    expect(screen.getByRole('menuitem', { name: /new agent/i })).toHaveAttribute('href', '/admin/agents/new')
  })

  it('surfaces the attention tiles an Editor + Reviewer can act on', async () => {
    renderOverview()
    expect(await screen.findByText(/awaiting your review/i)).toBeInTheDocument()
    expect(screen.getByText(/drafts in progress/i)).toBeInTheDocument()
  })

  it('hides the attention tiles from a Viewer (no edit / review)', async () => {
    mockPerms = { canEdit: () => false, canReview: () => false }
    renderOverview()
    await screen.findByText('12') // wait for stats to load
    expect(screen.queryByText(/awaiting your review/i)).not.toBeInTheDocument()
    expect(screen.queryByText(/drafts in progress/i)).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /^new/i })).not.toBeInTheDocument()
  })

  it('shows an onboarding empty state when the publisher has no resources', async () => {
    mockGET.mockImplementation((path: string) => {
      if (path === '/api/v1/publishers/{slug}/stats') {
        return Promise.resolve({
          data: { ...stats, mcp_servers: 0, agents: 0 },
        })
      }
      if (path === '/api/v1/publishers/{slug}/activity') return Promise.resolve({ data: { items: [] } })
      return Promise.resolve({ data: {} })
    })
    renderOverview()
    expect(await screen.findByText(/no resources yet/i)).toBeInTheDocument()
  })
})
