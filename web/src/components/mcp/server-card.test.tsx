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
  // After migration 000007 the registry stores a first-class `tools[]` field
  // on the latest version (distinct from `capabilities.tools`, which is the
  // MCP spec capability-negotiation flag). The chip renders only when the
  // array is present *and* non-empty — an absent field and a zero-length
  // array both hide it so a server that simply didn't declare tools is
  // not falsely advertised as tool-free.

  it('renders the tool count chip when latest_version.tools is a populated array', () => {
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
        tools: [{ name: 'read' }, { name: 'write' }, { name: 'list' }],
      },
    })
    renderWithRouter(<ServerCard server={server} />)
    expect(screen.getByText(/3 tools/)).toBeInTheDocument()
  })

  it('renders up to 3 tool-name chips alongside the count (mirrors agent-card skills+tags row)', () => {
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
        tools: [
          { name: 'read_file' },
          { name: 'write_file' },
          { name: 'list_directory' },
          { name: 'delete_file' }, // should NOT render — only first 3
          { name: 'watch_path' },
        ],
      },
    })
    renderWithRouter(<ServerCard server={server} />)
    // Count chip still renders.
    expect(screen.getByText(/5 tools/)).toBeInTheDocument()
    // First 3 tool names render as their own chips.
    expect(screen.getByText('read_file')).toBeInTheDocument()
    expect(screen.getByText('write_file')).toBeInTheDocument()
    expect(screen.getByText('list_directory')).toBeInTheDocument()
    // 4th + 5th tool names do NOT render on the card — they'd bloat the row.
    expect(screen.queryByText('delete_file')).not.toBeInTheDocument()
    expect(screen.queryByText('watch_path')).not.toBeInTheDocument()
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
        tools: [{ name: 'solo' }],
      },
    })
    renderWithRouter(<ServerCard server={server} />)
    // Exactly "1 tool" (no 's'). Use a regex anchored on word-boundary to
    // avoid matching "1 tools".
    expect(screen.getByText(/\b1 tool\b/)).toBeInTheDocument()
    expect(screen.queryByText(/1 tools/)).not.toBeInTheDocument()
  })

  it('hides the chip when latest_version.tools is absent', () => {
    // Default makeServer() has no `tools` field on latest_version.
    renderWithRouter(<ServerCard server={makeServer()} />)
    expect(screen.queryByText(/\btool(s)?\b/i)).not.toBeInTheDocument()
  })

  it('hides the chip when latest_version.tools is an empty array', () => {
    const server = makeServer({
      latest_version: {
        version: '2.0.0',
        runtime: 'http',
        protocol_version: '2025-03-26',
        packages: [
          { registryType: 'npm', identifier: '@acme/f', version: '2.0.0', transport: { type: 'stdio' } },
        ],
        tools: [],
      },
    })
    renderWithRouter(<ServerCard server={server} />)
    expect(screen.queryByText(/\btool(s)?\b/i)).not.toBeInTheDocument()
  })

  it('ignores capabilities.tools (the MCP capability-negotiation flag)', () => {
    // The old reading treated `capabilities.tools` as the tool list. It is
    // actually `{listChanged: bool}` and must never drive the chip — the
    // `tools[]` field is the only source of truth now.
    const server = makeServer({
      latest_version: {
        version: '2.0.0',
        runtime: 'http',
        protocol_version: '2025-03-26',
        packages: [
          { registryType: 'npm', identifier: '@acme/f', version: '2.0.0', transport: { type: 'stdio' } },
        ],
        capabilities: { tools: { listChanged: true } },
        // tools intentionally omitted
      },
    })
    renderWithRouter(<ServerCard server={server} />)
    expect(screen.queryByText(/\btool(s)?\b/i)).not.toBeInTheDocument()
  })
})
