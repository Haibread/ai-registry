import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { ServerCard } from './server-card'
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
    license: 'MIT',
    repo_url: 'https://github.com/acme/files',
    homepage_url: 'https://acme.dev/files',
    latest_version: {
      version: '2.0.0',
      // `runtime` in this codebase = MCP transport mechanism (see
      // server/internal/domain/mcp.go). Use the schema-valid enum, not a
      // language name — the as-MCPServer cast was hiding bogus 'node' values.
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

describe('ServerCard', () => {
  it('renders name, namespace/slug, version and runtime', () => {
    renderWithRouter(<ServerCard server={makeServer()} />)
    expect(screen.getByText('Files Server')).toBeInTheDocument()
    expect(screen.getByText('acme')).toBeInTheDocument()
    expect(screen.getByText('v2.0.0')).toBeInTheDocument()
    expect(screen.getByText('http')).toBeInTheDocument()
  })

  it('shows remote transport type and endpoint', () => {
    renderWithRouter(<ServerCard server={makeServer()} />)
    expect(screen.getByText('streamable_http')).toBeInTheDocument()
    expect(screen.getByText('https://acme.dev/mcp')).toBeInTheDocument()
  })

  it('renders license, repo and docs links when present', () => {
    renderWithRouter(<ServerCard server={makeServer()} />)
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
        // 'http' runtime used here so the runtime badge doesn't match /stdio/
        // in the assertion below — we're verifying the transport block is
        // suppressed when the package's transport.type is stdio.
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
    renderWithRouter(<ServerCard server={server} />)
    expect(screen.queryByText(/stdio/)).not.toBeInTheDocument()
    expect(screen.queryByText('https://acme.dev/mcp')).not.toBeInTheDocument()
  })

  it('omits repo and docs when their urls are missing', () => {
    const server = makeServer({ repo_url: undefined, homepage_url: undefined, license: undefined })
    renderWithRouter(<ServerCard server={server} />)
    expect(screen.queryByRole('link', { name: /view repository/i })).not.toBeInTheDocument()
    expect(screen.queryByRole('link', { name: /view documentation/i })).not.toBeInTheDocument()
  })

  // ── Tool count chip ──
  // `capabilities` is free-form JSON in the spec (decision F). The chip
  // must only render when a `tools` array is present *and* non-empty —
  // an absent field, a wrong-shape field, and a zero-length array all
  // hide the chip so a server that simply didn't populate the field is
  // not falsely advertised as tool-free.

  it('renders the tool count chip when capabilities.tools is a populated array', () => {
    const server = makeServer({
      latest_version: {
        version: '2.0.0',
        runtime: 'http',
        protocol_version: '2025-03-26',
        packages: [
          {
            registryType: 'npm',
            identifier: '@acme/files',
            version: '2.0.0',
            transport: { type: 'stdio' },
          },
        ],
        capabilities: {
          tools: [{ name: 'read' }, { name: 'write' }, { name: 'list' }],
        },
      },
    })
    renderWithRouter(<ServerCard server={server} />)
    expect(screen.getByText(/3 tools/)).toBeInTheDocument()
  })

  it('pluralises correctly for a single tool', () => {
    const server = makeServer({
      latest_version: {
        version: '2.0.0',
        runtime: 'http',
        protocol_version: '2025-03-26',
        packages: [
          { registryType: 'npm', identifier: '@acme/f', version: '2.0.0', transport: { type: 'stdio' } },
        ],
        capabilities: { tools: [{ name: 'solo' }] },
      },
    })
    renderWithRouter(<ServerCard server={server} />)
    // Exactly "1 tool" (no 's'). Use a regex anchored on word-boundary to
    // avoid matching "1 tools".
    expect(screen.getByText(/\b1 tool\b/)).toBeInTheDocument()
    expect(screen.queryByText(/1 tools/)).not.toBeInTheDocument()
  })

  it('hides the chip when capabilities is absent', () => {
    renderWithRouter(<ServerCard server={makeServer()} />)
    expect(screen.queryByText(/tool(s)?/i)).not.toBeInTheDocument()
  })

  it('hides the chip when capabilities.tools is absent', () => {
    const server = makeServer({
      latest_version: {
        version: '2.0.0',
        runtime: 'http',
        protocol_version: '2025-03-26',
        packages: [
          { registryType: 'npm', identifier: '@acme/f', version: '2.0.0', transport: { type: 'stdio' } },
        ],
        capabilities: { resources: [] },
      },
    })
    renderWithRouter(<ServerCard server={server} />)
    expect(screen.queryByText(/tool(s)?/i)).not.toBeInTheDocument()
  })

  it('hides the chip when capabilities.tools is the wrong shape', () => {
    const server = makeServer({
      latest_version: {
        version: '2.0.0',
        runtime: 'http',
        protocol_version: '2025-03-26',
        packages: [
          { registryType: 'npm', identifier: '@acme/f', version: '2.0.0', transport: { type: 'stdio' } },
        ],
        // Publishers sometimes ship tools as a map — we can't count
        // reliably, so the chip must hide rather than show "0 tools".
        capabilities: { tools: { read: {}, write: {} } },
      },
    })
    renderWithRouter(<ServerCard server={server} />)
    expect(screen.queryByText(/tool(s)?/i)).not.toBeInTheDocument()
  })

  it('hides the chip when capabilities.tools is an empty array', () => {
    const server = makeServer({
      latest_version: {
        version: '2.0.0',
        runtime: 'http',
        protocol_version: '2025-03-26',
        packages: [
          { registryType: 'npm', identifier: '@acme/f', version: '2.0.0', transport: { type: 'stdio' } },
        ],
        capabilities: { tools: [] },
      },
    })
    renderWithRouter(<ServerCard server={server} />)
    expect(screen.queryByText(/tool(s)?/i)).not.toBeInTheDocument()
  })
})
