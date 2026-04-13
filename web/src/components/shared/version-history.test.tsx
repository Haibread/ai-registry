import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { VersionHistory, VersionHistoryView } from './version-history'

// Mock api-client
const mockGET = vi.fn()
vi.mock('@/lib/api-client', () => ({
  getPublicClient: () => ({ GET: mockGET }),
}))

function wrapper({ children }: { children: React.ReactNode }) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return <QueryClientProvider client={qc}>{children}</QueryClientProvider>
}

describe('VersionHistory', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('shows loading skeletons initially', () => {
    mockGET.mockReturnValue(new Promise(() => {})) // never resolves
    const { container } = render(
      <VersionHistory type="mcp" namespace="acme" slug="test" />,
      { wrapper },
    )
    // Skeletons have rounded class
    const skeletons = container.querySelectorAll('.rounded')
    expect(skeletons.length).toBeGreaterThanOrEqual(2)
  })

  it('shows empty message when no versions', async () => {
    mockGET.mockResolvedValue({ data: { items: [] } })
    render(
      <VersionHistory type="mcp" namespace="acme" slug="test" />,
      { wrapper },
    )
    expect(await screen.findByText(/no versions published/i)).toBeInTheDocument()
  })

  it('renders version badges', async () => {
    mockGET.mockResolvedValue({
      data: {
        items: [
          { id: '1', version: '1.0.0', published_at: '2025-01-15T00:00:00Z', status: 'active' },
          { id: '2', version: '0.9.0', published_at: '2024-12-01T00:00:00Z', status: 'active' },
        ],
      },
    })
    render(
      <VersionHistory type="mcp" namespace="acme" slug="test" latestVersion="1.0.0" />,
      { wrapper },
    )
    expect(await screen.findByText('v1.0.0')).toBeInTheDocument()
    expect(screen.getByText('v0.9.0')).toBeInTheDocument()
  })

  it('marks latest version with a badge', async () => {
    mockGET.mockResolvedValue({
      data: {
        items: [
          { id: '1', version: '2.0.0', published_at: '2025-06-01T00:00:00Z', status: 'active' },
        ],
      },
    })
    render(
      <VersionHistory type="mcp" namespace="acme" slug="test" latestVersion="2.0.0" />,
      { wrapper },
    )
    expect(await screen.findByText('Latest')).toBeInTheDocument()
  })

  it('shows Draft for unpublished versions', async () => {
    mockGET.mockResolvedValue({
      data: {
        items: [
          { id: '1', version: '0.1.0', published_at: null, status: 'active' },
        ],
      },
    })
    render(
      <VersionHistory type="mcp" namespace="acme" slug="test" />,
      { wrapper },
    )
    expect(await screen.findByText('Draft')).toBeInTheDocument()
  })

  it('calls agent versions endpoint for type=agent', async () => {
    mockGET.mockResolvedValue({ data: { items: [] } })
    render(
      <VersionHistory type="agent" namespace="acme" slug="bot" />,
      { wrapper },
    )
    await screen.findByText(/no versions published/i)
    expect(mockGET).toHaveBeenCalledWith(
      '/api/v1/agents/{namespace}/{slug}/versions',
      expect.objectContaining({ params: { path: { namespace: 'acme', slug: 'bot' } } }),
    )
  })

  it('calls mcp versions endpoint for type=mcp', async () => {
    mockGET.mockResolvedValue({ data: { items: [] } })
    render(
      <VersionHistory type="mcp" namespace="acme" slug="srv" />,
      { wrapper },
    )
    await screen.findByText(/no versions published/i)
    expect(mockGET).toHaveBeenCalledWith(
      '/api/v1/mcp/servers/{namespace}/{slug}/versions',
      expect.objectContaining({ params: { path: { namespace: 'acme', slug: 'srv' } } }),
    )
  })
})

describe('VersionHistoryView compare mode', () => {
  const versions = [
    { id: 'v1', version: '1.0.0', runtime: 'node', published_at: '2025-01-15T00:00:00Z' },
    { id: 'v2', version: '0.9.0', runtime: 'python', published_at: '2024-12-01T00:00:00Z' },
  ]

  it('does not show compare toggle for a single version', () => {
    render(<VersionHistoryView versions={versions.slice(0, 1)} />)
    expect(screen.queryByRole('button', { name: /compare versions/i })).not.toBeInTheDocument()
  })

  it('shows compare toggle when there are 2+ versions', () => {
    render(<VersionHistoryView versions={versions} />)
    expect(screen.getByRole('button', { name: /compare versions/i })).toBeInTheDocument()
  })

  it('reveals diff after two versions are selected', () => {
    render(<VersionHistoryView versions={versions} />)
    fireEvent.click(screen.getByRole('button', { name: /compare versions/i }))
    // Select both via row click
    const rows = screen.getAllByRole('button').filter((b) => b.textContent?.includes('v'))
    expect(rows.length).toBeGreaterThanOrEqual(2)
    fireEvent.click(rows[0])
    fireEvent.click(rows[1])
    // Diff should render
    expect(screen.getByText(/comparing/i)).toBeInTheDocument()
  })

  it('exits compare mode and clears selection', () => {
    render(<VersionHistoryView versions={versions} />)
    const toggle = screen.getByRole('button', { name: /compare versions/i })
    fireEvent.click(toggle)
    fireEvent.click(screen.getByRole('button', { name: /exit compare/i }))
    // Back to the base label
    expect(screen.getByRole('button', { name: /compare versions/i })).toBeInTheDocument()
  })
})
