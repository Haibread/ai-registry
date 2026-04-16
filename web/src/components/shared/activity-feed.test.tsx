import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { ActivityFeed } from './activity-feed'

// Mock the public API client — each test drives it through `mockGET`.
const mockGET = vi.fn()
vi.mock('@/lib/api-client', () => ({
  getPublicClient: () => ({ GET: mockGET }),
}))

function wrapper({ children }: { children: React.ReactNode }) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return <QueryClientProvider client={qc}>{children}</QueryClientProvider>
}

// Fixed reference "now" so relative timestamps don't shift between test runs.
// We mock Date.now directly rather than using vi.useFakeTimers so that
// react-query's internal setTimeouts still work (fake timers block promise
// microtask draining).
const NOW = new Date('2026-04-16T12:00:00Z').getTime()

describe('ActivityFeed', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.spyOn(Date, 'now').mockReturnValue(NOW)
  })

  it('renders loading skeletons initially', () => {
    mockGET.mockReturnValue(new Promise(() => {})) // never resolves
    const { container } = render(
      <ActivityFeed resourceType="mcp" namespace="acme" slug="srv" />,
      { wrapper },
    )
    // There should be rounded skeleton bars rendered for the shimmer state.
    const skeletons = container.querySelectorAll('.rounded')
    expect(skeletons.length).toBeGreaterThanOrEqual(1)
  })

  it('renders an empty state when there are no events', async () => {
    mockGET.mockResolvedValue({
      data: { items: [], next_cursor: '' },
    })
    render(
      <ActivityFeed resourceType="mcp" namespace="acme" slug="srv" />,
      { wrapper },
    )
    expect(await screen.findByText(/no recorded activity/i)).toBeInTheDocument()
  })

  it('renders a populated feed with label, version, role, and relative time', async () => {
    mockGET.mockResolvedValue({
      data: {
        items: [
          {
            id: 'evt-1',
            action: 'mcp_server_version.published',
            actor_role: 'admin',
            version: '1.2.0',
            created_at: new Date(NOW - 3 * 60 * 60 * 1000).toISOString(), // 3h ago
          },
          {
            id: 'evt-2',
            action: 'mcp_server.created',
            actor_role: 'admin',
            created_at: new Date(NOW - 5 * 24 * 60 * 60 * 1000).toISOString(), // 5d
          },
        ],
        next_cursor: '',
      },
    })
    render(
      <ActivityFeed resourceType="mcp" namespace="acme" slug="srv" />,
      { wrapper },
    )

    expect(await screen.findByText(/version published/i)).toBeInTheDocument()
    expect(screen.getByText('v1.2.0')).toBeInTheDocument()
    expect(screen.getByText(/server created/i)).toBeInTheDocument()
    expect(screen.getByText('3h ago')).toBeInTheDocument()
    expect(screen.getByText('5d ago')).toBeInTheDocument()
  })

  it('surfaces scrubbed metadata (from/to/reason) as a compact summary', async () => {
    mockGET.mockResolvedValue({
      data: {
        items: [
          {
            id: 'evt-vis',
            action: 'mcp_server.visibility_changed',
            actor_role: 'admin',
            metadata: { from: 'private', to: 'public', reason: 'approved' },
            created_at: new Date(NOW - 1000).toISOString(),
          },
        ],
        next_cursor: '',
      },
    })
    render(
      <ActivityFeed resourceType="mcp" namespace="acme" slug="srv" />,
      { wrapper },
    )
    await screen.findByText(/visibility changed/i)
    // All three keys land in the same summary line.
    expect(screen.getByText(/from: private/i)).toBeInTheDocument()
    expect(screen.getByText(/to: public/i)).toBeInTheDocument()
    expect(screen.getByText(/reason: approved/i)).toBeInTheDocument()
  })

  it('never renders actor_email or actor_subject even if the server leaks them', async () => {
    // Defense-in-depth: even if an upstream regression exposed identity fields,
    // the component should not render them because it only reads whitelisted
    // fields off the DTO.
    mockGET.mockResolvedValue({
      data: {
        items: [
          {
            id: 'evt-leak',
            action: 'mcp_server.created',
            actor_role: 'admin',
            actor_email: 'secret@example.com',
            actor_subject: 'kc-subject-leaked',
            metadata: { client_ip: '10.0.0.1', internal_note: 'secret' },
            created_at: new Date(NOW - 1000).toISOString(),
          } as unknown as never,
        ],
        next_cursor: '',
      },
    })
    const { container } = render(
      <ActivityFeed resourceType="mcp" namespace="acme" slug="srv" />,
      { wrapper },
    )
    await screen.findByText(/server created/i)
    const rendered = container.textContent ?? ''
    expect(rendered).not.toContain('secret@example.com')
    expect(rendered).not.toContain('kc-subject-leaked')
    expect(rendered).not.toContain('10.0.0.1')
    expect(rendered).not.toContain('internal_note')
  })

  it('hides the Load more button when next_cursor is empty', async () => {
    mockGET.mockResolvedValue({
      data: {
        items: [
          {
            id: 'evt-1',
            action: 'mcp_server.created',
            actor_role: 'admin',
            created_at: new Date(NOW - 1000).toISOString(),
          },
        ],
        next_cursor: '',
      },
    })
    render(
      <ActivityFeed resourceType="mcp" namespace="acme" slug="srv" />,
      { wrapper },
    )
    await screen.findByText(/server created/i)
    expect(screen.queryByRole('button', { name: /load more/i })).not.toBeInTheDocument()
  })

  it('shows Load more when next_cursor is present and fetches the next page', async () => {
    mockGET
      .mockResolvedValueOnce({
        data: {
          items: [
            {
              id: 'evt-1',
              action: 'mcp_server.created',
              actor_role: 'admin',
              created_at: new Date(NOW - 1000).toISOString(),
            },
          ],
          next_cursor: 'CURSOR-PAGE-2',
        },
      })
      .mockResolvedValueOnce({
        data: {
          items: [
            {
              id: 'evt-2',
              action: 'mcp_server.updated',
              actor_role: 'admin',
              metadata: { field: 'description' },
              created_at: new Date(NOW - 2000).toISOString(),
            },
          ],
          next_cursor: '',
        },
      })

    render(
      <ActivityFeed resourceType="mcp" namespace="acme" slug="srv" />,
      { wrapper },
    )
    const loadMore = await screen.findByRole('button', { name: /load more/i })
    fireEvent.click(loadMore)

    // Page-2 item appears; page-1 item still on screen.
    await waitFor(() =>
      expect(screen.getByText(/metadata updated/i)).toBeInTheDocument(),
    )
    expect(screen.getByText(/server created/i)).toBeInTheDocument()

    // And the second call used the cursor from the first page's response.
    expect(mockGET).toHaveBeenLastCalledWith(
      '/api/v1/mcp/servers/{namespace}/{slug}/activity',
      expect.objectContaining({
        params: expect.objectContaining({
          query: expect.objectContaining({ cursor: 'CURSOR-PAGE-2' }),
        }),
      }),
    )
  })

  it('targets the agent activity endpoint when resourceType=agent', async () => {
    mockGET.mockResolvedValue({ data: { items: [], next_cursor: '' } })
    render(
      <ActivityFeed resourceType="agent" namespace="acme" slug="bot" />,
      { wrapper },
    )
    await screen.findByText(/no recorded activity/i)
    expect(mockGET).toHaveBeenCalledWith(
      '/api/v1/agents/{namespace}/{slug}/activity',
      expect.objectContaining({
        params: expect.objectContaining({
          path: { namespace: 'acme', slug: 'bot' },
        }),
      }),
    )
  })
})
