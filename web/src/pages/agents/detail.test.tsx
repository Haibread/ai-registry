import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'

// jsdom has no IntersectionObserver; StickyDetailHeader uses it.
beforeEach(() => {
  ;(globalThis as unknown as { IntersectionObserver: unknown }).IntersectionObserver = vi.fn(function () {
    return { observe: vi.fn(), disconnect: vi.fn(), unobserve: vi.fn() }
  })
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
  useCatalogClient: () => ({ api: { GET: mockGET, POST: mockPOST }, viewer: 'anonymous', ready: true }),
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

function renderDetail(ns = 'anthropic', slug = 'code-review', hash = '') {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={[`/agents/${ns}/${slug}${hash}`]}>
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

  it('marks a verified, featured agent and hides the published status', async () => {
    renderDetail()
    await screen.findByRole('heading', { name: /code review agent/i })
    expect(screen.getAllByText('Verified').length).toBeGreaterThan(0)
    expect(screen.getByText('Featured')).toBeInTheDocument()
    expect(screen.queryByText('published')).not.toBeInTheDocument()
  })

  it('links to the A2A agent card', async () => {
    renderDetail()
    const link = await screen.findByRole('link', { name: /a2a agent card/i })
    expect(link).toHaveAttribute(
      'href',
      '/agents/anthropic/code-review/.well-known/agent-card.json',
    )
  })

  it('shows the endpoint URL and auth scheme in the Quick connect card', async () => {
    renderDetail()
    const connect = await screen.findByRole('region', { name: 'Quick connect' })
    expect(within(connect).getByText('https://agents.anthropic.com/code-review')).toBeInTheDocument()
    expect(within(connect).getByRole('button', { name: /copy endpoint url/i })).toBeInTheDocument()
    expect(within(connect).getByText('Bearer')).toBeInTheDocument()
  })

  it('lists protocol version and IO modes in the Details card', async () => {
    renderDetail()
    const details = await screen.findByRole('region', { name: 'Details' })
    expect(within(details).getByText('A2A protocol')).toBeInTheDocument()
    expect(within(details).getByText('0.3.0')).toBeInTheDocument()
    expect(within(details).getByText('Input')).toBeInTheDocument()
    expect(within(details).getByText('Output')).toBeInTheDocument()
    expect(within(details).getByText('10 views · 3 installs')).toBeInTheDocument()
  })

  it('renders the tab navigation: Overview, Skills (N), Usage, Versions', async () => {
    renderDetail()
    await screen.findByRole('heading', { name: /code review agent/i })
    expect(screen.getByRole('tab', { name: /overview/i })).toBeInTheDocument()
    expect(screen.getByRole('tab', { name: /skills \(2\)/i })).toBeInTheDocument()
    expect(screen.getByRole('tab', { name: /usage/i })).toBeInTheDocument()
    expect(screen.getByRole('tab', { name: /versions/i })).toBeInTheDocument()
    expect(screen.queryByRole('tab', { name: /json/i })).not.toBeInTheDocument()
  })

  it('opens the Usage tab from a legacy #connect link', async () => {
    renderDetail('anthropic', 'code-review', '#connect')
    await screen.findByRole('heading', { name: /code review agent/i })
    expect(screen.getByRole('tab', { name: /usage/i })).toHaveAttribute('aria-selected', 'true')
  })

  it('opens the Usage tab from the Quick connect card', async () => {
    const user = userEvent.setup()
    renderDetail()
    await screen.findByRole('heading', { name: /code review agent/i })
    await user.click(screen.getByRole('button', { name: /set up a client/i }))
    expect(screen.getByRole('tab', { name: /usage/i })).toHaveAttribute('aria-selected', 'true')
  })

  it('lists skills on the Skills tab', async () => {
    const user = userEvent.setup()
    renderDetail()
    await screen.findByRole('heading', { name: /code review agent/i })

    await user.click(screen.getByRole('tab', { name: /skills/i }))
    expect(await screen.findByText('Review Pull Request')).toBeInTheDocument()
    expect(screen.getByText('Suggest Fix')).toBeInTheDocument()
  })

  it('says when the agent needs no authentication', async () => {
    primeGET({
      ...AGENT,
      latest_version: { ...AGENT.latest_version, authentication: [] },
    })
    renderDetail()
    const connect = await screen.findByRole('region', { name: 'Quick connect' })
    expect(within(connect).getByText('No authentication')).toBeInTheDocument()
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

    for (const name of [/skills/i, /usage/i, /versions/i]) {
      await user.click(screen.getByRole('tab', { name }))
      expect(activePanelClass()).toMatch(/\bmt-6\b/)
    }
  })

  it('renders the README as markdown, with raw HTML inert and javascript: links neutralised', async () => {
    primeGET({
      ...AGENT,
      readme: '## Usage\n\n<img src=x onerror="alert(1)">\n\n[docs](https://example.com/docs) [evil](javascript:alert(1))',
    })
    const { container } = renderDetail()

    expect(await screen.findByRole('heading', { level: 2, name: 'Usage' })).toBeInTheDocument()
    expect(container.querySelector('.prose img')).toBeNull()
    expect(screen.getByText('<img src=x onerror="alert(1)">')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'docs' })).toHaveAttribute('href', 'https://example.com/docs')
    expect(screen.getByText('evil').closest('a')).toHaveAttribute('href', '')
  })

  it('renders no README block when the entry has none', async () => {
    primeGET({ ...AGENT, readme: '' })
    const { container } = renderDetail()

    await screen.findByRole('heading', { level: 1 })
    expect(container.querySelector('.prose')).toBeNull()
  })
})
