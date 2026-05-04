/**
 * admin-sidebar.test.tsx
 *
 * Tests for the active-route detection logic in AdminSidebar.
 *
 * The sidebar has two matching modes:
 *  - exact=true  (Dashboard): only highlights when pathname === "/admin"
 *  - exact=false (all others): highlights when pathname.startsWith(href)
 */

import { render, screen, waitFor } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { AdminSidebar } from './admin-sidebar'

vi.mock('@/auth/AuthContext', () => ({
  useAuth: () => ({ accessToken: 'test-token' }),
}))

const mockGET = vi.fn()
vi.mock('@/lib/api-client', () => ({
  useAuthClient: () => ({ GET: mockGET }),
}))

beforeEach(() => {
  vi.clearAllMocks()
  // Default: empty queue → no badge.
  mockGET.mockResolvedValue({ data: { items: [] } })
})

function renderSidebar(pathname: string) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={[pathname]}>
        <AdminSidebar pathname={pathname} />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

function linkClass(label: string): string {
  return screen.getByRole('link', { name: new RegExp(label, 'i') }).className
}

const ACTIVE_CLASS = 'bg-accent text-accent-foreground'
const INACTIVE_CLASS = 'text-muted-foreground'

describe('AdminSidebar — active route detection', () => {
  it('highlights Dashboard only on exact /admin match', () => {
    renderSidebar('/admin')
    expect(linkClass('Dashboard')).toContain(ACTIVE_CLASS)
  })

  it('does NOT highlight Dashboard on /admin/mcp', () => {
    renderSidebar('/admin/mcp')
    expect(linkClass('Dashboard')).toContain(INACTIVE_CLASS)
    expect(linkClass('Dashboard')).not.toContain(ACTIVE_CLASS)
  })

  it('highlights MCP Servers on /admin/mcp', () => {
    renderSidebar('/admin/mcp')
    expect(linkClass('MCP Servers')).toContain(ACTIVE_CLASS)
  })

  it('highlights MCP Servers on a nested path like /admin/mcp/acme/my-server', () => {
    renderSidebar('/admin/mcp/acme/my-server')
    expect(linkClass('MCP Servers')).toContain(ACTIVE_CLASS)
  })

  it('highlights MCP Servers on /admin/mcp/new', () => {
    renderSidebar('/admin/mcp/new')
    expect(linkClass('MCP Servers')).toContain(ACTIVE_CLASS)
  })

  it('highlights Agents on /admin/agents', () => {
    renderSidebar('/admin/agents')
    expect(linkClass('Agents')).toContain(ACTIVE_CLASS)
  })

  it('highlights Agents on a nested agent path', () => {
    renderSidebar('/admin/agents/acme/my-agent')
    expect(linkClass('Agents')).toContain(ACTIVE_CLASS)
  })

  it('highlights Publishers on /admin/publishers', () => {
    renderSidebar('/admin/publishers')
    expect(linkClass('Publishers')).toContain(ACTIVE_CLASS)
  })

  it('highlights API Keys on /admin/api-keys', () => {
    renderSidebar('/admin/api-keys')
    expect(linkClass('API Keys')).toContain(ACTIVE_CLASS)
  })

  it('highlights Activity on /admin/audit', () => {
    renderSidebar('/admin/audit')
    expect(linkClass('Activity')).toContain(ACTIVE_CLASS)
  })

  it('only highlights one item at a time', () => {
    renderSidebar('/admin/mcp')
    const activeLinks = screen.getAllByRole('link').filter(el => el.className.includes(ACTIVE_CLASS))
    expect(activeLinks).toHaveLength(1)
  })

  it('renders all nav items', () => {
    renderSidebar('/admin')
    expect(screen.getByRole('link', { name: /dashboard/i })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /publishers/i })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /mcp servers/i })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /agents/i })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /reports/i })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /activity/i })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /api keys/i })).toBeInTheDocument()
  })
})

describe('AdminSidebar — review queue badge', () => {
  it('renders no badge when the queue is empty', async () => {
    mockGET.mockResolvedValue({ data: { items: [] } })
    renderSidebar('/admin')
    // Wait for the query to settle so the `null` return is stable.
    await waitFor(() => expect(mockGET).toHaveBeenCalled())
    expect(screen.queryByLabelText(/pending review/i)).not.toBeInTheDocument()
  })

  it('renders a count badge when the queue has items', async () => {
    mockGET.mockResolvedValue({
      data: { items: Array.from({ length: 3 }, () => ({})) },
    })
    renderSidebar('/admin')
    expect(await screen.findByLabelText(/3 items pending review/i)).toBeInTheDocument()
  })

  it('clamps overflow at 99+ when next_cursor is set', async () => {
    mockGET.mockResolvedValue({
      data: {
        items: Array.from({ length: 99 }, () => ({})),
        next_cursor: 'more',
      },
    })
    renderSidebar('/admin')
    expect(await screen.findByText('99+')).toBeInTheDocument()
  })

  it('uses singular "item" copy for exactly 1 pending', async () => {
    mockGET.mockResolvedValue({ data: { items: [{}] } })
    renderSidebar('/admin')
    expect(await screen.findByLabelText(/^1 item pending review$/i)).toBeInTheDocument()
  })
})
