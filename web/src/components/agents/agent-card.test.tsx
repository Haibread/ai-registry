import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { AgentCard } from './agent-card'
import type { components } from '@/lib/schema'

type Agent = components['schemas']['Agent']

function makeAgent(overrides: Partial<Agent> = {}): Agent {
  return {
    id: '01H0000000000000000000',
    namespace: 'acme',
    slug: 'bot',
    name: 'Acme Bot',
    description: 'A helpful agent',
    status: 'published',
    verified: false,
    view_count: 1234,
    updated_at: '2025-01-15T00:00:00Z',
    created_at: '2025-01-01T00:00:00Z',
    latest_version: {
      version: '1.2.3',
      skills: [
        { id: 's1', name: 'Search', description: 'search', tags: ['web', 'nlp'] },
        { id: 's2', name: 'Code', description: 'code', tags: ['nlp'] },
      ],
      endpoint_url: 'https://agent.example.com/a2a',
    },
    ...overrides,
  } as Agent
}

function renderWithRouter(ui: React.ReactNode) {
  // The card resolves instance-tag display names via useInstanceTags, so it
  // needs a QueryClient; retries off so a test never hangs on fetch failures.
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>{ui}</MemoryRouter>
    </QueryClientProvider>,
  )
}

describe('AgentCard', () => {
  it('titles the card with a heading that links to the detail page', () => {
    renderWithRouter(<AgentCard agent={makeAgent()} />)
    const heading = screen.getByRole('heading', { level: 3, name: 'Acme Bot' })
    expect(heading.querySelector('a')).toHaveAttribute('href', '/agents/acme/bot')
  })

  it('renders name, namespace/slug and version', () => {
    renderWithRouter(<AgentCard agent={makeAgent()} />)
    expect(screen.getByRole('link', { name: 'Acme Bot' })).toHaveAttribute('href', '/agents/acme/bot')
    expect(screen.getByRole('link', { name: 'acme' })).toHaveAttribute('href', '/agents/acme')
    expect(screen.getByText(/\/bot/)).toBeInTheDocument()
    expect(screen.getByText('v1.2.3')).toBeInTheDocument()
  })

  it('renders the description and skill count', () => {
    renderWithRouter(<AgentCard agent={makeAgent()} />)
    expect(screen.getByText('A helpful agent')).toBeInTheDocument()
    expect(screen.getByText('2 skills')).toBeInTheDocument()
  })

  it('leaves endpoint and outbound links to the detail page', () => {
    renderWithRouter(<AgentCard agent={makeAgent()} />)
    expect(screen.queryByText('https://agent.example.com/a2a')).not.toBeInTheDocument()
    expect(screen.queryByRole('link', { name: /json|agent card/i })).not.toBeInTheDocument()
  })

  it('marks verified and featured agents', () => {
    renderWithRouter(<AgentCard agent={makeAgent({ verified: true, featured: true })} />)
    expect(screen.getByRole('img', { name: 'Verified' })).toBeInTheDocument()
    expect(screen.getByText('Featured')).toBeInTheDocument()
  })

  it('shows the status only when it is not published', () => {
    const { unmount } = renderWithRouter(<AgentCard agent={makeAgent()} />)
    expect(screen.queryByText('published')).not.toBeInTheDocument()
    unmount()
    renderWithRouter(<AgentCard agent={makeAgent({ status: 'deprecated' })} />)
    expect(screen.getByText('deprecated')).toBeInTheDocument()
  })

  it('omits the skills chip when the version declares none', () => {
    const agent = makeAgent({
      latest_version: { version: '0.1.0', skills: [], endpoint_url: '' },
      description: undefined,
    })
    renderWithRouter(<AgentCard agent={agent} />)
    expect(screen.queryByText(/skills?/i)).not.toBeInTheDocument()
    expect(screen.queryByText(/A helpful agent/)).not.toBeInTheDocument()
  })

  it('omits the skills chip when the skills field is entirely absent', () => {
    const agent = makeAgent({
      latest_version: { version: '0.1.0', endpoint_url: 'https://a.example/a2a' },
    })
    renderWithRouter(<AgentCard agent={agent} />)
    expect(screen.queryByText(/skills?/i)).not.toBeInTheDocument()
  })
})
