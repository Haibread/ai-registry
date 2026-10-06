import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { UsageTab } from '@/components/mcp/usage-tab'
import type { components } from '@/lib/schema'

type MCPServer = components['schemas']['MCPServer']

const server: MCPServer = {
  id: 'srv-1',
  namespace: 'acme',
  slug: 'github-tools',
  name: 'GitHub Tools',
  visibility: 'public',
  status: 'published',
  featured: false,
  verified: false,
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
  latest_version: {
    version: '1.4.2',
    runtime: 'stdio',
    protocol_versions: ['2025-06-18'],
    packages: [
      { registryType: 'npm', identifier: '@acme/github-tools', version: '1.4.2', transport: { type: 'stdio' } },
    ],
  },
}

describe('UsageTab', () => {
  it('shows the generated instructions when the publisher wrote none', () => {
    render(<UsageTab server={server} version={server.latest_version} />)
    expect(screen.getByRole('heading', { name: 'Connect' })).toBeInTheDocument()
    expect(screen.queryByText(/written by/i)).not.toBeInTheDocument()
  })

  it('renders the publisher Markdown with placeholders substituted, generated content folded', () => {
    render(<UsageTab server={{ ...server, usage_markdown: '## Getting started\n\nInstall v{{version}}.' }} version={server.latest_version} />)
    expect(screen.getByRole('heading', { name: 'Getting started' })).toBeInTheDocument()
    expect(screen.getByText('Install v1.4.2.')).toBeInTheDocument()
    expect(screen.getByText(/written by/i)).toHaveTextContent('Written by acme')
    const fold = screen.getByText('Generated configuration').closest('details')
    expect(fold).not.toHaveAttribute('open')
  })

  it('fills placeholders from the version shown, not only the latest published one', () => {
    const { latest_version: draft, ...unpublished } = server
    render(<UsageTab server={{ ...unpublished, usage_markdown: 'Try v{{version}}' }} version={{ ...draft!, version: '0.1.0' }} />)
    expect(screen.getByText('Try v0.1.0')).toBeInTheDocument()
  })

  it('shows the empty state with neither Markdown nor packages', () => {
    render(<UsageTab server={{ ...server, latest_version: undefined }} version={undefined} />)
    expect(screen.getByText('No packages available')).toBeInTheDocument()
  })

  it('shows the Markdown alone when there is nothing generated', () => {
    render(<UsageTab server={{ ...server, latest_version: undefined, usage_markdown: 'Ask us for access.' }} version={undefined} />)
    expect(screen.getByText('Ask us for access.')).toBeInTheDocument()
    expect(screen.queryByText('Generated configuration')).not.toBeInTheDocument()
  })
})
