import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'

vi.mock('@/auth/AuthContext', () => ({
  useAuth: () => ({ accessToken: null, login: vi.fn(), logout: vi.fn(), loginError: null }),
}))
vi.mock('@/components/providers', () => ({
  useTheme: () => ({ theme: 'light', setTheme: vi.fn() }),
}))

const mockGET = vi.fn()
vi.mock('@/lib/api-client', () => ({
  getPublicClient: () => ({ GET: mockGET }),
}))

import ChangelogPage from './changelog'

function renderPage() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter>
        <ChangelogPage />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

describe('ChangelogPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders the heading', () => {
    mockGET.mockResolvedValue({ data: { items: [] } })
    renderPage()
    expect(screen.getByRole('heading', { name: /changelog/i, level: 1 })).toBeInTheDocument()
  })

  it('shows an empty state when there are no entries', async () => {
    mockGET.mockResolvedValue({ data: { items: [] } })
    renderPage()
    expect(await screen.findByText(/no recent releases/i)).toBeInTheDocument()
  })

  it('renders grouped entries by day', async () => {
    mockGET.mockResolvedValue({
      data: {
        items: [
          {
            resource_type: 'mcp_server',
            namespace: 'acme',
            slug: 'srv-a',
            name: 'Server A',
            version: '1.0.0',
            published_at: '2026-04-10T10:00:00Z',
          },
          {
            resource_type: 'agent',
            namespace: 'acme',
            slug: 'bot-a',
            name: 'Bot A',
            version: '0.5.0',
            published_at: '2026-04-10T09:00:00Z',
          },
          {
            resource_type: 'mcp_server',
            namespace: 'acme',
            slug: 'srv-b',
            name: 'Server B',
            version: '2.0.0',
            published_at: '2026-04-09T11:00:00Z',
          },
        ],
      },
    })
    renderPage()
    await waitFor(() => {
      expect(screen.getByText('Server A')).toBeInTheDocument()
      expect(screen.getByText('Bot A')).toBeInTheDocument()
      expect(screen.getByText('Server B')).toBeInTheDocument()
    })
    expect(screen.getAllByText(/v\d/).length).toBeGreaterThanOrEqual(3)
  })

  it('links each entry to its detail page', async () => {
    mockGET.mockResolvedValue({
      data: {
        items: [
          {
            resource_type: 'mcp_server',
            namespace: 'acme',
            slug: 'srv-a',
            name: 'Server A',
            version: '1.0.0',
            published_at: '2026-04-10T10:00:00Z',
          },
        ],
      },
    })
    renderPage()
    const link = await screen.findByRole('link', { name: 'Server A' })
    expect(link).toHaveAttribute('href', '/mcp/acme/srv-a')
  })

  it('shows an error state on failure', async () => {
    mockGET.mockRejectedValueOnce(new Error('boom'))
    renderPage()
    expect(await screen.findByText(/failed to load the changelog/i)).toBeInTheDocument()
  })
})
