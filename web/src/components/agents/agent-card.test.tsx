import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
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
  return render(<MemoryRouter>{ui}</MemoryRouter>)
}

describe('AgentCard', () => {
  it('renders name, namespace/slug and version', () => {
    renderWithRouter(<AgentCard agent={makeAgent()} />)
    expect(screen.getByText('Acme Bot')).toBeInTheDocument()
    expect(screen.getByText('acme')).toBeInTheDocument()
    expect(screen.getByText(/\/bot/)).toBeInTheDocument()
    expect(screen.getByText('v1.2.3')).toBeInTheDocument()
  })

  it('renders description and endpoint url when present', () => {
    renderWithRouter(<AgentCard agent={makeAgent()} />)
    expect(screen.getByText('A helpful agent')).toBeInTheDocument()
    expect(screen.getByText('https://agent.example.com/a2a')).toBeInTheDocument()
  })

  it('shows skill count and unique tags (up to 3)', () => {
    renderWithRouter(<AgentCard agent={makeAgent()} />)
    expect(screen.getByText(/2 skills/)).toBeInTheDocument()
    expect(screen.getByText('web')).toBeInTheDocument()
    // 'nlp' deduped
    expect(screen.getAllByText('nlp')).toHaveLength(1)
  })

  it('links to the agent detail and JSON API', () => {
    renderWithRouter(<AgentCard agent={makeAgent()} />)
    const detail = screen.getByRole('link', { name: 'Acme Bot' })
    expect(detail).toHaveAttribute('href', '/agents/acme/bot')
    const json = screen.getByRole('link', { name: /view json api response/i })
    expect(json).toHaveAttribute('href', '/api/v1/agents/acme/bot')
  })

  it('omits skill badge and endpoint block when absent', () => {
    // `endpoint_url` is required as `string` in the schema, but the card
    // treats falsy values as "no endpoint" — empty string triggers that path
    // without violating the type.
    const agent = makeAgent({
      latest_version: { version: '0.1.0', skills: [], endpoint_url: '' },
      description: undefined,
    })
    renderWithRouter(<AgentCard agent={agent} />)
    expect(screen.queryByText(/skills?/i)).not.toBeInTheDocument()
    expect(screen.queryByText(/A helpful agent/)).not.toBeInTheDocument()
  })
})
