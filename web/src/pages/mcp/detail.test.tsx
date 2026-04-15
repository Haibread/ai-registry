import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'

// jsdom has no IntersectionObserver; StickyDetailHeader uses it. Stub a noop
// implementation so the detail page mounts without crashing.
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

// Header uses auth + theme.
vi.mock('@/auth/AuthContext', () => ({
  useAuth: () => ({ accessToken: null, login: vi.fn(), logout: vi.fn(), loginError: null }),
}))
vi.mock('@/components/providers', () => ({
  useTheme: () => ({ theme: 'light', setTheme: vi.fn() }),
}))

// Stub the fire-and-forget tracking hooks so they never touch the API client.
vi.mock('@/hooks/use-record-event', () => ({
  useRecordView: vi.fn(),
  useRecordCopy: vi.fn(() => vi.fn()),
}))

const mockGET = vi.fn()
const mockPOST = vi.fn().mockResolvedValue({})
vi.mock('@/lib/api-client', () => ({
  getPublicClient: () => ({ GET: mockGET, POST: mockPOST }),
}))

import MCPDetailPage from './detail'

// Minimal MCP server payload shaped like the typed OpenAPI response. The
// component only reads a subset of fields so we don't need the full schema.
const STDIO_SERVER = {
  id: 'srv-1',
  namespace: 'anthropic',
  slug: 'filesystem',
  name: 'Filesystem MCP Server',
  description: 'Gives Claude access to the local file system.',
  status: 'published',
  visibility: 'public',
  verified: true,
  featured: true,
  tags: ['filesystem', 'storage'],
  readme: '# Filesystem\n\nA test readme.',
  license: 'MIT',
  homepage_url: 'https://example.com',
  repo_url: 'https://github.com/anthropics/mcp-filesystem',
  view_count: 42,
  copy_count: 7,
  created_at: '2025-01-01T00:00:00Z',
  updated_at: '2025-02-01T00:00:00Z',
  latest_version: {
    version: '1.0.0',
    runtime: 'stdio',
    protocol_version: '2025-03-26',
    published_at: '2025-02-01T00:00:00Z',
    capabilities: {
      tools: { listChanged: true },
      resources: {},
    },
    packages: [
      {
        registryType: 'npm',
        identifier: '@anthropic/mcp-filesystem',
        version: '1.0.0',
        transport: { type: 'stdio' },
      },
    ],
  },
}

const REMOTE_SERVER = {
  ...STDIO_SERVER,
  slug: 'computer-use',
  name: 'Computer Use',
  latest_version: {
    ...STDIO_SERVER.latest_version,
    runtime: 'sse',
    packages: [
      {
        registryType: 'npm',
        identifier: '@anthropic/mcp-computer-use',
        version: '1.0.0',
        transport: { type: 'sse', url: 'https://mcp.anthropic.com/computer-use/sse' },
      },
    ],
  },
}

function renderDetail(ns = 'anthropic', slug = 'filesystem') {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={[`/mcp/${ns}/${slug}`]}>
        <Routes>
          <Route path="/mcp/:ns/:slug" element={<MCPDetailPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

// Default: GET returns the stdio server unless a test overrides it. Auxiliary
// queries (publisher sidebar, version history, related entries) return empty.
function primeGET(server: unknown) {
  mockGET.mockImplementation((path: string) => {
    if (path.includes('/mcp/servers/{namespace}/{slug}') && !path.includes('versions')) {
      return Promise.resolve({ data: server })
    }
    if (path.includes('/publishers/')) {
      return Promise.resolve({
        data: { id: 'p1', slug: 'anthropic', name: 'Anthropic', verified: true },
      })
    }
    return Promise.resolve({ data: { items: [], total_count: 0 } })
  })
}

describe('MCPDetailPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockPOST.mockResolvedValue({})
    primeGET(STDIO_SERVER)
  })

  it('renders the server name and namespace/slug identifier', async () => {
    renderDetail()
    expect(
      await screen.findByRole('heading', { name: /filesystem mcp server/i }),
    ).toBeInTheDocument()
    // Identifier row carries "anthropic/filesystem"
    expect(screen.getByText(/^\/filesystem$/)).toBeInTheDocument()
  })

  it('renders verified + status badges from the payload', async () => {
    renderDetail()
    expect(await screen.findByText(/verified/i)).toBeInTheDocument()
    // "Published" appears in multiple places (StatusBadge, StatTile label,
    // formatted date cell). Assert that at least one is present — we just
    // care that the status surfaced somewhere on the page.
    const publishedHits = screen.getAllByText(/published/i)
    expect(publishedHits.length).toBeGreaterThan(0)
  })

  it('renders the Overview section header as "Runtime & Capabilities" for stdio servers', async () => {
    renderDetail()
    expect(await screen.findByText('Runtime & Capabilities')).toBeInTheDocument()
    // stdio servers do NOT get a Connection header or endpoint URL tile
    expect(screen.queryByText('Connection & Runtime')).not.toBeInTheDocument()
    expect(screen.queryByText('Endpoint URL')).not.toBeInTheDocument()
  })

  it('renders runtime and protocol version tiles', async () => {
    renderDetail()
    expect(await screen.findByText('Runtime')).toBeInTheDocument()
    expect(screen.getByText('stdio')).toBeInTheDocument()
    expect(screen.getByText('Protocol version')).toBeInTheDocument()
    expect(screen.getByText('2025-03-26')).toBeInTheDocument()
  })

  it('renders the tab navigation: Overview, Installation, Tools, Versions, JSON', async () => {
    renderDetail()
    await screen.findByRole('heading', { name: /filesystem mcp server/i })
    expect(screen.getByRole('tab', { name: /overview/i })).toBeInTheDocument()
    expect(screen.getByRole('tab', { name: /installation/i })).toBeInTheDocument()
    expect(screen.getByRole('tab', { name: /^tools/i })).toBeInTheDocument()
    expect(screen.getByRole('tab', { name: /versions/i })).toBeInTheDocument()
    expect(screen.getByRole('tab', { name: /json/i })).toBeInTheDocument()
  })

  it('switches to the Installation tab and shows the package identifier', async () => {
    const user = userEvent.setup()
    renderDetail()
    await screen.findByRole('heading', { name: /filesystem mcp server/i })

    await user.click(screen.getByRole('tab', { name: /installation/i }))

    // The install panel renders the package identifier + version. The
    // MCPConfigGenerator below it also shows the same identifier in the
    // generated config — at least one match means the tab is mounted.
    const hits = await screen.findAllByText(/@anthropic\/mcp-filesystem@1\.0\.0/)
    expect(hits.length).toBeGreaterThan(0)
  })

  it('renders the "Not Found" empty state when the API returns no body', async () => {
    // The detail page treats both `null` and an error as "not found".
    // Use `null` here so TanStack Query doesn't warn about undefined data.
    mockGET.mockResolvedValue({ data: null })
    renderDetail('nobody', 'nothing')
    expect(await screen.findByText(/server not found/i)).toBeInTheDocument()
  })
})

describe('MCPDetailPage — remote transport', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockPOST.mockResolvedValue({})
    primeGET(REMOTE_SERVER)
  })

  it('renders the "Connection & Runtime" header for remote transports', async () => {
    renderDetail('anthropic', 'computer-use')
    expect(await screen.findByText('Connection & Runtime')).toBeInTheDocument()
    expect(screen.queryByText('Runtime & Capabilities')).not.toBeInTheDocument()
  })

  it('surfaces the endpoint URL as a hero row in the Overview', async () => {
    renderDetail('anthropic', 'computer-use')
    expect(await screen.findByText('Endpoint URL')).toBeInTheDocument()
    const link = screen.getByRole('link', {
      name: /mcp\.anthropic\.com\/computer-use\/sse/,
    })
    expect(link).toHaveAttribute('href', 'https://mcp.anthropic.com/computer-use/sse')
  })

  it('replaces the Runtime tile with a Transport tile for remote servers', async () => {
    renderDetail('anthropic', 'computer-use')
    expect(await screen.findByText('Transport')).toBeInTheDocument()
    // The literal word "Runtime" (the stdio tile label) should be gone.
    // Note: "Runtime & Capabilities" header is also gone because we're remote.
    const runtimeLabels = screen.queryAllByText('Runtime')
    expect(runtimeLabels.length).toBe(0)
  })

  it('shows an MCP-spec authentication tile for remote servers', async () => {
    renderDetail('anthropic', 'computer-use')
    expect(await screen.findByText('Authentication')).toBeInTheDocument()
    expect(screen.getByText(/per mcp spec \(oauth 2\.1\)/i)).toBeInTheDocument()
  })

  it('stacks multiple endpoint rows when the server ships multiple remote packages', async () => {
    const multi = {
      ...REMOTE_SERVER,
      latest_version: {
        ...REMOTE_SERVER.latest_version,
        packages: [
          {
            registryType: 'npm',
            identifier: '@acme/a',
            version: '1.0.0',
            transport: { type: 'sse', url: 'https://a.example.com/sse' },
          },
          {
            registryType: 'oci',
            identifier: 'ghcr.io/acme/b',
            version: '1.0.0',
            transport: { type: 'http', url: 'https://b.example.com' },
          },
        ],
      },
    }
    primeGET(multi)
    renderDetail('anthropic', 'computer-use')

    // Both URL tile labels should appear, tagged with their transport type.
    expect(await screen.findByText(/endpoint url \(sse\)/i)).toBeInTheDocument()
    expect(screen.getByText(/endpoint url \(http\)/i)).toBeInTheDocument()
    // And both URLs should be reachable links.
    expect(screen.getByRole('link', { name: /a\.example\.com\/sse/ })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /b\.example\.com/ })).toBeInTheDocument()
  })
})

describe('MCPDetailPage — tools tab', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockPOST.mockResolvedValue({})
  })

  it('shows the tool count on the tab trigger when tools are present', async () => {
    primeGET({
      ...STDIO_SERVER,
      latest_version: {
        ...STDIO_SERVER.latest_version,
        tools: [
          { name: 'read_file', description: 'Read a file' },
          { name: 'write_file', description: 'Write a file' },
        ],
      },
    })
    renderDetail()
    await screen.findByRole('heading', { name: /filesystem mcp server/i })
    // Tab label is "Tools (2)" when populated.
    expect(screen.getByRole('tab', { name: /tools \(2\)/i })).toBeInTheDocument()
  })

  it('renders a card per tool with name and description', async () => {
    const user = userEvent.setup()
    primeGET({
      ...STDIO_SERVER,
      latest_version: {
        ...STDIO_SERVER.latest_version,
        tools: [
          { name: 'read_file', description: 'Read a file from disk' },
          { name: 'write_file', description: 'Write a file to disk' },
        ],
      },
    })
    renderDetail()
    await screen.findByRole('heading', { name: /filesystem mcp server/i })

    await user.click(screen.getByRole('tab', { name: /^tools/i }))

    expect(screen.getByText('read_file')).toBeInTheDocument()
    expect(screen.getByText('write_file')).toBeInTheDocument()
    expect(screen.getByText(/read a file from disk/i)).toBeInTheDocument()
    expect(screen.getByText(/write a file to disk/i)).toBeInTheDocument()
  })

  it('renders the empty state when tools is absent or empty', async () => {
    const user = userEvent.setup()
    // Default STDIO_SERVER has no `tools` field.
    primeGET(STDIO_SERVER)
    renderDetail()
    await screen.findByRole('heading', { name: /filesystem mcp server/i })

    await user.click(screen.getByRole('tab', { name: /^tools/i }))

    expect(screen.getByText(/no tools declared/i)).toBeInTheDocument()
  })

  it('renders annotation badges for truthy boolean annotations', async () => {
    const user = userEvent.setup()
    primeGET({
      ...STDIO_SERVER,
      latest_version: {
        ...STDIO_SERVER.latest_version,
        tools: [
          {
            name: 'delete_file',
            description: 'Delete a file',
            annotations: { destructive: true, idempotent: false },
          },
        ],
      },
    })
    renderDetail()
    await screen.findByRole('heading', { name: /filesystem mcp server/i })

    await user.click(screen.getByRole('tab', { name: /^tools/i }))

    // Truthy annotation surfaces as a badge; falsy one is hidden.
    expect(screen.getByText('destructive')).toBeInTheDocument()
    expect(screen.queryByText('idempotent')).not.toBeInTheDocument()
  })
})

describe('MCPDetailPage — tab spacing', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockPOST.mockResolvedValue({})
    primeGET(STDIO_SERVER)
  })

  // Radix lazy-mounts only the active panel, so we have to click through the
  // tabs to verify that each TabsContent carries the `mt-6` override. The
  // override matters: without it, non-overview tabs inherit Radix's default
  // `mt-2` and sit visibly closer to the tab list than the overview panel.
  it('applies mt-6 to every TabsContent so non-overview tabs match the overview rhythm', async () => {
    const user = userEvent.setup()
    renderDetail()
    await screen.findByRole('heading', { name: /filesystem mcp server/i })

    const activePanelClass = () =>
      document.querySelector('[role="tabpanel"][data-state="active"]')?.className ?? ''

    // Overview is the default active tab.
    expect(activePanelClass()).toMatch(/\bmt-6\b/)

    for (const name of [/installation/i, /^tools/i, /versions/i, /json/i]) {
      await user.click(screen.getByRole('tab', { name }))
      expect(activePanelClass()).toMatch(/\bmt-6\b/)
    }
  })
})
