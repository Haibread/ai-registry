import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { ServerListItem } from './server-list-item'
import type { components } from '@/lib/schema'

type MCPServer = components['schemas']['MCPServer']

function makeServer(overrides: Partial<MCPServer> = {}): MCPServer {
  return {
    id: '01H0000000000000000000',
    namespace: 'acme',
    slug: 'files',
    name: 'Files Server',
    description: 'A file server',
    status: 'published',
    verified: true,
    view_count: 42,
    updated_at: '2025-01-15T00:00:00Z',
    created_at: '2025-01-01T00:00:00Z',
    latest_version: {
      version: '2.0.0',
      runtime: 'http',
      protocol_version: '2025-03-26',
      packages: [
        {
          registryType: 'npm',
          identifier: '@acme/files',
          version: '2.0.0',
          transport: { type: 'streamable_http', url: 'https://acme.dev/mcp' },
        },
      ],
    },
    ...overrides,
  } as MCPServer
}

function renderWithRouter(ui: React.ReactNode) {
  return render(<MemoryRouter>{ui}</MemoryRouter>)
}

describe('ServerListItem', () => {
  it('renders name, namespace/slug, and version', () => {
    renderWithRouter(<ServerListItem server={makeServer()} />)
    expect(screen.getByText('Files Server')).toBeInTheDocument()
    expect(screen.getByText('acme/files')).toBeInTheDocument()
    expect(screen.getByText('v2.0.0')).toBeInTheDocument()
  })

  it('links to the server detail page', () => {
    renderWithRouter(<ServerListItem server={makeServer()} />)
    const link = screen.getByRole('link', { name: /files server/i })
    expect(link).toHaveAttribute('href', '/mcp/acme/files')
  })

  it('renders the description truncated to one line', () => {
    renderWithRouter(<ServerListItem server={makeServer()} />)
    expect(screen.getByText('A file server')).toBeInTheDocument()
  })

  it('omits the description block when absent', () => {
    renderWithRouter(<ServerListItem server={makeServer({ description: undefined })} />)
    expect(screen.queryByText('A file server')).not.toBeInTheDocument()
  })

  it('renders a "remote" tag for remote-transport packages', () => {
    renderWithRouter(<ServerListItem server={makeServer()} />)
    expect(screen.getByText('remote')).toBeInTheDocument()
  })

  it('omits the "remote" tag for stdio-only packages', () => {
    const server = makeServer({
      latest_version: {
        version: '1.0.0',
        runtime: 'http',
        protocol_version: '2025-03-26',
        packages: [
          {
            registryType: 'pypi',
            identifier: 'acme-files',
            version: '1.0.0',
            transport: { type: 'stdio' },
          },
        ],
      },
    })
    renderWithRouter(<ServerListItem server={server} />)
    expect(screen.queryByText('remote')).not.toBeInTheDocument()
  })
})
