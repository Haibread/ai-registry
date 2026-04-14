import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { ServerCard } from './server-card'

function makeServer(overrides: any = {}) {
  return {
    id: '01H0000000000000000000',
    namespace: 'acme',
    slug: 'files',
    name: 'Files Server',
    description: 'A file server',
    status: 'active',
    verified: true,
    view_count: 42,
    updated_at: '2025-01-15T00:00:00Z',
    created_at: '2025-01-01T00:00:00Z',
    license: 'MIT',
    repo_url: 'https://github.com/acme/files',
    homepage_url: 'https://acme.dev/files',
    latest_version: {
      version: '2.0.0',
      runtime: 'node',
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
  }
}

function renderWithRouter(ui: React.ReactNode) {
  return render(<MemoryRouter>{ui}</MemoryRouter>)
}

describe('ServerCard', () => {
  it('renders name, namespace/slug, version and runtime', () => {
    renderWithRouter(<ServerCard server={makeServer() as any} />)
    expect(screen.getByText('Files Server')).toBeInTheDocument()
    expect(screen.getByText('acme')).toBeInTheDocument()
    expect(screen.getByText('v2.0.0')).toBeInTheDocument()
    expect(screen.getByText('node')).toBeInTheDocument()
  })

  it('shows remote transport type and endpoint', () => {
    renderWithRouter(<ServerCard server={makeServer() as any} />)
    expect(screen.getByText('streamable_http')).toBeInTheDocument()
    expect(screen.getByText('https://acme.dev/mcp')).toBeInTheDocument()
  })

  it('renders license, repo and docs links when present', () => {
    renderWithRouter(<ServerCard server={makeServer() as any} />)
    expect(screen.getByText('MIT')).toBeInTheDocument()
    const repo = screen.getByRole('link', { name: /view repository/i })
    expect(repo).toHaveAttribute('href', 'https://github.com/acme/files')
    const docs = screen.getByRole('link', { name: /view documentation/i })
    expect(docs).toHaveAttribute('href', 'https://acme.dev/files')
  })

  it('does not render transport block for stdio', () => {
    const server = makeServer({
      latest_version: {
        version: '1.0.0',
        runtime: 'python',
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
    renderWithRouter(<ServerCard server={server as any} />)
    expect(screen.queryByText(/stdio/)).not.toBeInTheDocument()
    expect(screen.queryByText('https://acme.dev/mcp')).not.toBeInTheDocument()
  })

  it('omits repo and docs when their urls are missing', () => {
    const server = makeServer({ repo_url: undefined, homepage_url: undefined, license: undefined })
    renderWithRouter(<ServerCard server={server as any} />)
    expect(screen.queryByRole('link', { name: /view repository/i })).not.toBeInTheDocument()
    expect(screen.queryByRole('link', { name: /view documentation/i })).not.toBeInTheDocument()
  })
})
