import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'

// jsdom has no IntersectionObserver; StickyDetailHeader uses it. Stub a noop
// implementation so the detail page mounts without crashing.
beforeEach(() => {
  ;(globalThis as unknown as { IntersectionObserver: unknown }).IntersectionObserver = vi.fn(function () {
    return { observe: vi.fn(), disconnect: vi.fn(), unobserve: vi.fn() }
  })
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
  useCatalogClient: () => ({ api: { GET: mockGET, POST: mockPOST }, viewer: 'anonymous', ready: true }),
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
    protocol_versions: ['2025-03-26'],
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

function renderDetail(ns = 'anthropic', slug = 'filesystem', hash = '') {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={[`/mcp/${ns}/${slug}${hash}`]}>
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

  it('marks a verified, featured server and hides the published status', async () => {
    renderDetail()
    await screen.findByRole('heading', { name: /filesystem mcp server/i })
    expect(screen.getAllByText('Verified').length).toBeGreaterThan(0)
    expect(screen.getByText('Featured')).toBeInTheDocument()
    expect(screen.queryByText('published')).not.toBeInTheDocument()
  })

  it('shows the run command of a local server in the Quick connect card', async () => {
    renderDetail()
    const connect = await screen.findByRole('region', { name: 'Quick connect' })
    expect(within(connect).getByText('npx -y @anthropic/mcp-filesystem')).toBeInTheDocument()
    expect(within(connect).getByText(/runs locally over stdio/i)).toBeInTheDocument()
  })

  it('lists runtime, protocol versions and license in the Details card', async () => {
    renderDetail()
    const details = await screen.findByRole('region', { name: 'Details' })
    expect(within(details).getByText('Runtime')).toBeInTheDocument()
    expect(within(details).getByText('stdio')).toBeInTheDocument()
    expect(within(details).getByText('2025-03-26')).toBeInTheDocument()
    expect(within(details).getByText('MIT')).toBeInTheDocument()
    expect(within(details).getByText('42 views · 7 installs')).toBeInTheDocument()
  })

  it('renders the tab navigation: Overview, Usage, Tools, Versions', async () => {
    renderDetail()
    await screen.findByRole('heading', { name: /filesystem mcp server/i })
    expect(screen.getByRole('tab', { name: /overview/i })).toBeInTheDocument()
    expect(screen.getByRole('tab', { name: /usage/i })).toBeInTheDocument()
    expect(screen.getByRole('tab', { name: /^tools/i })).toBeInTheDocument()
    expect(screen.getByRole('tab', { name: /versions/i })).toBeInTheDocument()
    expect(screen.queryByRole('tab', { name: /json/i })).not.toBeInTheDocument()
  })

  it('keeps the raw API response one click away on the Overview', async () => {
    renderDetail()
    await screen.findByRole('heading', { name: /filesystem mcp server/i })
    expect(screen.getByText('Raw API response')).toBeInTheDocument()
  })

  it('opens the Usage tab from the Quick connect card', async () => {
    const user = userEvent.setup()
    renderDetail()
    await screen.findByRole('heading', { name: /filesystem mcp server/i })
    await user.click(screen.getByRole('button', { name: /set up in your client/i }))
    expect(screen.getByRole('tab', { name: /usage/i })).toHaveAttribute('aria-selected', 'true')
  })

  it('switches to the Usage tab and shows the package identifier', async () => {
    const user = userEvent.setup()
    renderDetail()
    await screen.findByRole('heading', { name: /filesystem mcp server/i })

    await user.click(screen.getByRole('tab', { name: /usage/i }))

    // The install panel renders the package identifier + version. The
    // MCPConfigGenerator below it also shows the same identifier in the
    // generated config — at least one match means the tab is mounted.
    const hits = await screen.findAllByText(/@anthropic\/mcp-filesystem@1\.0\.0/)
    expect(hits.length).toBeGreaterThan(0)
  })

  it('opens the Usage tab from a legacy #installation link', async () => {
    renderDetail('anthropic', 'filesystem', '#installation')
    await screen.findByRole('heading', { name: /filesystem mcp server/i })
    expect(screen.getByRole('tab', { name: /usage/i })).toHaveAttribute('aria-selected', 'true')
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

  it('shows the endpoint URL with its transport in the Quick connect card', async () => {
    renderDetail('anthropic', 'computer-use')
    const connect = await screen.findByRole('region', { name: 'Quick connect' })
    expect(within(connect).getByText('https://mcp.anthropic.com/computer-use/sse')).toBeInTheDocument()
    expect(within(connect).getByText('sse')).toBeInTheDocument()
    expect(within(connect).getByRole('button', { name: /copy endpoint url/i })).toBeInTheDocument()
    expect(within(connect).getByText(/auth per mcp spec \(oauth 2\.1\)/i)).toBeInTheDocument()
  })

  it('labels the runtime as the transport for remote servers', async () => {
    renderDetail('anthropic', 'computer-use')
    const details = await screen.findByRole('region', { name: 'Details' })
    expect(within(details).getByText('Transport')).toBeInTheDocument()
    expect(within(details).queryByText('Runtime')).not.toBeInTheDocument()
  })

  it('lists every endpoint when the server ships multiple remote packages', async () => {
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

    const connect = await screen.findByRole('region', { name: 'Quick connect' })
    expect(within(connect).getByText('https://a.example.com/sse')).toBeInTheDocument()
    expect(within(connect).getByText('https://b.example.com')).toBeInTheDocument()
    expect(within(connect).getAllByRole('button', { name: /copy endpoint url/i })).toHaveLength(2)
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

  it('lists every tool and opens the first one', async () => {
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

    const list = screen.getByRole('list', { name: 'Tools' })
    expect(within(list).getByText('read_file')).toBeInTheDocument()
    expect(within(list).getByText('write_file')).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'read_file' })).toBeInTheDocument()
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

  it('explains the MCP behaviour hints of the selected tool', async () => {
    const user = userEvent.setup()
    primeGET({
      ...STDIO_SERVER,
      latest_version: {
        ...STDIO_SERVER.latest_version,
        tools: [
          {
            name: 'delete_file',
            description: 'Delete a file',
            annotations: { destructiveHint: true, idempotentHint: false },
          },
        ],
      },
    })
    renderDetail()
    await screen.findByRole('heading', { name: /filesystem mcp server/i })

    await user.click(screen.getByRole('tab', { name: /^tools/i }))

    expect(screen.getByText('May delete or overwrite data.')).toBeInTheDocument()
    expect(screen.getByText('Repeating a call may have further effect.')).toBeInTheDocument()
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

    for (const name of [/usage/i, /^tools/i, /versions/i]) {
      await user.click(screen.getByRole('tab', { name }))
      expect(activePanelClass()).toMatch(/\bmt-6\b/)
    }
  })

})

describe('MCPDetailPage — README', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockPOST.mockResolvedValue({})
  })

  it('renders the README as markdown, with raw HTML inert and javascript: links neutralised', async () => {
    primeGET({
      ...STDIO_SERVER,
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
    primeGET({ ...STDIO_SERVER, readme: '' })
    const { container } = renderDetail()

    await screen.findByRole('heading', { level: 1 })
    expect(container.querySelector('.prose')).toBeNull()
  })
})

describe('MCPDetailPage — unpublished entry seen by a member', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockPOST.mockResolvedValue({})
  })

  const { latest_version: published, ...withoutVersion } = STDIO_SERVER
  const DRAFT_SERVER = { ...withoutVersion, status: 'draft', visibility: 'private' }

  function primeDraft() {
    mockGET.mockImplementation((path: string) => {
      if (path.endsWith('/versions')) {
        return Promise.resolve({
          data: {
            items: [{
              ...published,
              id: 'v1',
              version: '0.1.0',
              published_at: undefined,
              status: 'active',
              tools: [{ name: 'draft_tool', description: 'Only in the draft' }],
            }],
          },
        })
      }
      if (path.includes('/mcp/servers/{namespace}/{slug}') && !path.includes('versions')) {
        return Promise.resolve({ data: DRAFT_SERVER })
      }
      return Promise.resolve({ data: { items: [], total_count: 0 } })
    })
  }

  it('says who can see it and links to its admin page', async () => {
    primeDraft()
    renderDetail()
    const banner = await screen.findByRole('status')
    expect(banner).toHaveTextContent(/only members of anthropic can see this server/i)
    expect(await screen.findByText(/showing unpublished version/i)).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /manage/i })).toHaveAttribute('href', '/admin/mcp/anthropic/filesystem')
  })

  it('shows the tools of the newest unpublished version', async () => {
    const user = userEvent.setup()
    primeDraft()
    renderDetail()
    await screen.findByText(/showing unpublished version/i)
    await user.click(screen.getByRole('tab', { name: /tools \(1\)/i }))
    expect(within(screen.getByRole('list', { name: 'Tools' })).getByText('draft_tool')).toBeInTheDocument()
  })

  it('does not count the visit as a catalog view', async () => {
    const { useRecordView } = await import('@/hooks/use-record-event')
    primeDraft()
    renderDetail()
    await screen.findByRole('heading', { name: /filesystem mcp server/i })
    expect(useRecordView).not.toHaveBeenCalledWith('mcp', 'anthropic', 'filesystem')
  })

  it('shows no banner on a published public entry', async () => {
    primeGET(STDIO_SERVER)
    renderDetail()
    await screen.findByRole('heading', { name: /filesystem mcp server/i })
    expect(screen.queryByText(/can see this server/i)).not.toBeInTheDocument()
  })
})
