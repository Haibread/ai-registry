import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { RelatedEntries } from './related-entries'

const mockGET = vi.fn()
vi.mock('@/lib/api-client', () => ({
  getPublicClient: () => ({ GET: mockGET }),
}))

function wrap(ui: React.ReactNode) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return (
    <MemoryRouter>
      <QueryClientProvider client={qc}>{ui}</QueryClientProvider>
    </MemoryRouter>
  )
}

const mcpBase = {
  id: 'id1',
  namespace: 'acme',
  slug: 'other',
  name: 'Other Server',
  status: 'published',
  verified: false,
  view_count: 0,
  updated_at: '2025-01-01T00:00:00Z',
  created_at: '2025-01-01T00:00:00Z',
  latest_version: {
    version: '1.0.0',
    runtime: 'http',
    protocol_version: '2025-03-26',
    packages: [],
  },
}

const agentBase = {
  id: 'aid1',
  namespace: 'acme',
  slug: 'other-bot',
  name: 'Other Bot',
  status: 'published',
  verified: false,
  view_count: 0,
  updated_at: '2025-01-01T00:00:00Z',
  created_at: '2025-01-01T00:00:00Z',
  latest_version: { version: '1.0.0', endpoint_url: '', skills: [] },
}

describe('RelatedEntries', () => {
  beforeEach(() => {
    mockGET.mockReset()
  })

  it('renders nothing when there are no related items', async () => {
    mockGET.mockResolvedValue({ data: { items: [] } })
    const { container } = render(
      wrap(<RelatedEntries type="mcp" namespace="acme" currentSlug="me" />),
    )
    await waitFor(() => {
      expect(container.innerHTML).toBe('')
    })
  })

  it('filters out the current slug for mcp', async () => {
    mockGET.mockResolvedValue({
      data: {
        items: [
          { ...mcpBase, id: '1', slug: 'me', name: 'Me Server' },
          { ...mcpBase, id: '2', slug: 'other', name: 'Other Server' },
        ],
      },
    })
    render(wrap(<RelatedEntries type="mcp" namespace="acme" currentSlug="me" />))
    expect(await screen.findByText(/More from acme/)).toBeInTheDocument()
    expect(screen.getByText('Other Server')).toBeInTheDocument()
    expect(screen.queryByText('Me Server')).not.toBeInTheDocument()
  })

  it('renders AgentCards for type=agent', async () => {
    mockGET.mockResolvedValue({
      data: {
        items: [{ ...agentBase, id: 'a1', slug: 'other-bot', name: 'Other Bot' }],
      },
    })
    render(wrap(<RelatedEntries type="agent" namespace="acme" currentSlug="mine" />))
    expect(await screen.findByText('Other Bot')).toBeInTheDocument()
  })

  it('limits results to 3', async () => {
    mockGET.mockResolvedValue({
      data: {
        items: [
          { ...mcpBase, id: '1', slug: 's1', name: 'S1' },
          { ...mcpBase, id: '2', slug: 's2', name: 'S2' },
          { ...mcpBase, id: '3', slug: 's3', name: 'S3' },
          { ...mcpBase, id: '4', slug: 's4', name: 'S4' },
        ],
      },
    })
    render(wrap(<RelatedEntries type="mcp" namespace="acme" currentSlug="me" />))
    expect(await screen.findByText('S1')).toBeInTheDocument()
    expect(screen.getByText('S2')).toBeInTheDocument()
    expect(screen.getByText('S3')).toBeInTheDocument()
    expect(screen.queryByText('S4')).not.toBeInTheDocument()
  })
})
