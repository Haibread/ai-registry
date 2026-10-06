import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
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
      protocol_versions: ['2025-03-26'],
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
  // The card resolves instance-tag display names via useInstanceTags, so it
  // needs a QueryClient; retries off so a test never hangs on fetch failures.
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>{ui}</MemoryRouter>
    </QueryClientProvider>,
  )
}

describe('ServerCard', () => {
  it('titles the card with a heading that links to the detail page', () => {
    renderWithRouter(<ServerCard server={makeServer()} />)
    const heading = screen.getByRole('heading', { level: 3, name: 'Files Server' })
    expect(heading.querySelector('a')).toHaveAttribute('href', '/mcp/acme/files')
  })

  it('renders name, namespace/slug and version', () => {
    renderWithRouter(<ServerCard server={makeServer()} />)
    expect(screen.getByRole('link', { name: 'Files Server' })).toHaveAttribute('href', '/mcp/acme/files')
    expect(screen.getByRole('link', { name: 'acme' })).toHaveAttribute('href', '/mcp/acme')
    expect(screen.getByText('v2.0.0')).toBeInTheDocument()
  })

  it('labels a remote server with its transport', () => {
    renderWithRouter(<ServerCard server={makeServer()} />)
    expect(screen.getByText('remote · streamable_http')).toBeInTheDocument()
  })

  it('labels a local server with its package ecosystem', () => {
    const server = makeServer({
      latest_version: {
        version: '1.0.0',
        runtime: 'stdio',
        protocol_versions: ['2025-03-26'],
        packages: [
          { registryType: 'pypi', identifier: 'acme-files', version: '1.0.0', transport: { type: 'stdio' } },
        ],
      },
    })
    renderWithRouter(<ServerCard server={server} />)
    expect(screen.getByText('local · pip')).toBeInTheDocument()
  })

  it('leaves endpoint and outbound links to the detail page', () => {
    renderWithRouter(<ServerCard server={makeServer()} />)
    expect(screen.queryByText('https://acme.dev/mcp')).not.toBeInTheDocument()
    expect(screen.queryByRole('link', { name: /repository|documentation|json/i })).not.toBeInTheDocument()
  })

  it('marks verified and featured servers', () => {
    renderWithRouter(<ServerCard server={makeServer({ featured: true })} />)
    expect(screen.getByRole('img', { name: 'Verified' })).toBeInTheDocument()
    expect(screen.getByText('Featured')).toBeInTheDocument()
  })

  it('shows the status only when it is not published', () => {
    const { unmount } = renderWithRouter(<ServerCard server={makeServer()} />)
    expect(screen.queryByText('published')).not.toBeInTheDocument()
    unmount()
    renderWithRouter(<ServerCard server={makeServer({ status: 'deprecated' })} />)
    expect(screen.getByText('deprecated')).toBeInTheDocument()
  })

  it('shows two tags and counts the rest', () => {
    renderWithRouter(<ServerCard server={makeServer({ tags: ['db', 'sql', 'cloud', 'ops'] })} />)
    expect(screen.getByText('db')).toBeInTheDocument()
    expect(screen.getByText('sql')).toBeInTheDocument()
    expect(screen.queryByText('cloud')).not.toBeInTheDocument()
    expect(screen.getByText('+2')).toBeInTheDocument()
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
        protocol_versions: ['2025-03-26'],
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

  it('pluralises correctly for a single tool', () => {
    const server = makeServer({
      latest_version: {
        version: '2.0.0',
        runtime: 'http',
        protocol_versions: ['2025-03-26'],
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
        protocol_versions: ['2025-03-26'],
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
        protocol_versions: ['2025-03-26'],
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
