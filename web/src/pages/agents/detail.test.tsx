import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'

// jsdom has no IntersectionObserver; StickyDetailHeader uses it.
beforeEach(() => {
  ;(globalThis as unknown as { IntersectionObserver: unknown }).IntersectionObserver = vi.fn(() => ({
    observe: vi.fn(),
    disconnect: vi.fn(),
    unobserve: vi.fn(),
  }))
})
afterEach(() => {
  delete (globalThis as unknown as { IntersectionObserver?: unknown }).IntersectionObserver
})

vi.mock('@/auth/AuthContext', () => ({
  useAuth: () => ({ accessToken: null, login: vi.fn(), logout: vi.fn(), loginError: null }),
}))
vi.mock('@/components/providers', () => ({
  useTheme: () => ({ theme: 'light', setTheme: vi.fn() }),
}))

vi.mock('@/hooks/use-record-event', () => ({
  useRecordView: vi.fn(),
  useRecordCopy: vi.fn(() => vi.fn()),
}))

const mockGET = vi.fn()
const mockPOST = vi.fn().mockResolvedValue({})
vi.mock('@/lib/api-client', () => ({
  getPublicClient: () => ({ GET: mockGET, POST: mockPOST }),
}))

import AgentDetailPage from './detail'

const AGENT = {
  id: 'ag-1',
  namespace: 'anthropic',
  slug: 'code-review',
  name: 'Code Review Agent',
  description: 'Reviews pull requests.',
  status: 'published',
  visibility: 'public',
  verified: true,
  featured: true,
  tags: ['code-review', 'devtools'],
  readme: '# Code Review\n\nTest readme.',
  view_count: 10,
  copy_count: 3,
  created_at: '2025-01-01T00:00:00Z',
  updated_at: '2025-02-01T00:00:00Z',
  latest_version: {
    version: '1.0.0',
    endpoint_url: 'https://agents.anthropic.com/code-review',
    protocol_version: '0.3.0',
    published_at: '2025-02-01T00:00:00Z',
    default_input_modes: ['text/plain'],
    default_output_modes: ['text/plain', 'image/png'],
    skills: [
      {
        id: 'review-pr',
        name: 'Review Pull Request',
        description: 'Reads a PR diff and posts inline comments.',
        tags: ['git'],
      },
      {
        id: 'suggest-fix',
        name: 'Suggest Fix',
        description: 'Generates a code fix for a flagged issue.',
        tags: ['code-quality'],
      },
    ],
    authentication: [{ scheme: 'Bearer' }],
  },
}

function renderDetail(ns = 'anthropic', slug = 'code-review') {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={[`/agents/${ns}/${slug}`]}>
        <Routes>
          <Route path="/agents/:ns/:slug" element={<AgentDetailPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

function primeGET(agent: unknown) {
  mockGET.mockImplementation((path: string) => {
    if (path.includes('/agents/{namespace}/{slug}') && !path.includes('versions')) {
      return Promise.resolve({ data: agent })
    }
    if (path.includes('/publishers/')) {
      return Promise.resolve({
        data: { id: 'p1', slug: 'anthropic', name: 'Anthropic', verified: true },
      })
    }
    return Promise.resolve({ data: { items: [], total_count: 0 } })
  })
}

describe('AgentDetailPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockPOST.mockResolvedValue({})
    primeGET(AGENT)
  })

  it('renders the agent name and namespace/slug identifier', async () => {
    renderDetail()
    expect(
      await screen.findByRole('heading', { name: /code review agent/i }),
    ).toBeInTheDocument()
    expect(screen.getByText(/^\/code-review$/)).toBeInTheDocument()
  })

  it('renders the verified badge and status', async () => {
    renderDetail()
    expect(await screen.findByText(/verified/i)).toBeInTheDocument()
    expect(screen.getAllByText(/published/i).length).toBeGreaterThan(0)
  })

  it('links to the A2A agent card', async () => {
    renderDetail()
    const link = await screen.findByRole('link', { name: /a2a agent card/i })
    expect(link).toHaveAttribute(
      'href',
      '/agents/anthropic/code-review/.well-known/agent-card.json',
    )
  })

  it('renders the Connection card with the endpoint URL as a hero row', async () => {
    renderDetail()
    expect(await screen.findByText('Connection')).toBeInTheDocument()
    expect(screen.getByText('Endpoint URL')).toBeInTheDocument()
    const link = screen.getByRole('link', { name: /agents\.anthropic\.com\/code-review/ })
    expect(link).toHaveAttribute('href', 'https://agents.anthropic.com/code-review')
  })

  it('renders the A2A protocol version tile', async () => {
    renderDetail()
    expect(await screen.findByText('A2A Protocol')).toBeInTheDocument()
    expect(screen.getByText('0.3.0')).toBeInTheDocument()
  })

  it('renders input and output mode tiles', async () => {
    renderDetail()
    expect(await screen.findByText('Input modes')).toBeInTheDocument()
    expect(screen.getByText('Output modes')).toBeInTheDocument()
  })

  it('renders the Authentication tile with the declared scheme', async () => {
    renderDetail()
    expect(await screen.findByText('Authentication')).toBeInTheDocument()
    expect(screen.getByText('Bearer')).toBeInTheDocument()
  })

  it('renders the tab navigation: Overview, Skills (N), Connect, Versions, JSON', async () => {
    renderDetail()
    await screen.findByRole('heading', { name: /code review agent/i })
    expect(screen.getByRole('tab', { name: /overview/i })).toBeInTheDocument()
    // Skill tab shows the count in parentheses.
    expect(screen.getByRole('tab', { name: /skills \(2\)/i })).toBeInTheDocument()
    expect(screen.getByRole('tab', { name: /connect/i })).toBeInTheDocument()
    expect(screen.getByRole('tab', { name: /versions/i })).toBeInTheDocument()
    expect(screen.getByRole('tab', { name: /json/i })).toBeInTheDocument()
  })

  it('lists skills on the Skills tab', async () => {
    const user = userEvent.setup()
    renderDetail()
    await screen.findByRole('heading', { name: /code review agent/i })

    await user.click(screen.getByRole('tab', { name: /skills/i }))
    expect(await screen.findByText('Review Pull Request')).toBeInTheDocument()
    expect(screen.getByText('Suggest Fix')).toBeInTheDocument()
  })

  it('shows Public when authentication list is empty', async () => {
    primeGET({
      ...AGENT,
      latest_version: { ...AGENT.latest_version, authentication: [] },
    })
    renderDetail()
    // Wait for Connection card, then look inside the Auth tile.
    await screen.findByText('Authentication')
    expect(screen.getByText('Public')).toBeInTheDocument()
  })

  it('renders the "Agent not found" empty state when the API returns no body', async () => {
    mockGET.mockResolvedValue({ data: null })
    renderDetail('nobody', 'nothing')
    expect(await screen.findByText(/agent not found/i)).toBeInTheDocument()
  })

  it('applies mt-6 to every TabsContent so non-overview tabs match the overview rhythm', async () => {
    const user = userEvent.setup()
    renderDetail()
    await screen.findByRole('heading', { name: /code review agent/i })

    const activePanelClass = () =>
      document.querySelector('[role="tabpanel"][data-state="active"]')?.className ?? ''

    // Overview is the default active tab.
    expect(activePanelClass()).toMatch(/\bmt-6\b/)

    for (const name of [/skills/i, /connect/i, /versions/i, /json/i]) {
      await user.click(screen.getByRole('tab', { name }))
      expect(activePanelClass()).toMatch(/\bmt-6\b/)
    }
  })
})
