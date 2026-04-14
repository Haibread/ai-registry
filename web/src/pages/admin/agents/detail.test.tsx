import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor, fireEvent } from '@testing-library/react'
import { MemoryRouter, Routes, Route } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'

vi.mock('@/auth/AuthContext', () => ({
  useAuth: () => ({ accessToken: 'test-token' }),
}))

const mockGET = vi.fn()
const mockPOST = vi.fn()
const mockPATCH = vi.fn()
const mockDELETE = vi.fn()
vi.mock('@/lib/api-client', () => ({
  useAuthClient: () => ({ GET: mockGET, POST: mockPOST, PATCH: mockPATCH, DELETE: mockDELETE }),
}))

import AdminAgentDetail from './detail'

function renderPage() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={['/admin/agents/acme/example-agent']}>
        <Routes>
          <Route path="/admin/agents/:ns/:slug" element={<AdminAgentDetail />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

const sampleAgent = {
  id: '01HAGT1',
  namespace: 'acme',
  slug: 'example-agent',
  name: 'Example Agent',
  description: 'An example agent',
  status: 'published',
  visibility: 'public',
  created_at: '2026-04-01T10:00:00Z',
  updated_at: '2026-04-02T10:00:00Z',
  latest_version: {
    version: '0.1.0',
    endpoint_url: 'https://agent.example.test/a2a',
    protocol_version: '2025-06-18',
    published_at: '2026-04-02T10:00:00Z',
    default_input_modes: ['text'],
    default_output_modes: ['text'],
    authentication: [{ scheme: 'Bearer' }],
    skills: [
      {
        id: 'greet',
        name: 'Greeter',
        description: 'Says hi',
        tags: ['social'],
        examples: ['Hello world'],
      },
    ],
  },
}

describe('AdminAgentDetail', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockGET.mockResolvedValue({ data: sampleAgent })
    mockPOST.mockResolvedValue({})
    mockPATCH.mockResolvedValue({})
    mockDELETE.mockResolvedValue({})
  })

  it('fetches the agent detail on mount', async () => {
    renderPage()
    await waitFor(() => {
      expect(mockGET).toHaveBeenCalledWith('/api/v1/agents/{namespace}/{slug}', {
        params: { path: { namespace: 'acme', slug: 'example-agent' } },
      })
    })
  })

  it('renders the heading and the agent metadata', async () => {
    renderPage()
    expect(await screen.findByRole('heading', { name: 'Example Agent' })).toBeInTheDocument()
    expect(screen.getByText('An example agent')).toBeInTheDocument()
    expect(screen.getByText('v0.1.0')).toBeInTheDocument()
    expect(screen.getByText('https://agent.example.test/a2a')).toBeInTheDocument()
  })

  it('renders skills from the latest version', async () => {
    renderPage()
    expect(await screen.findByText('Greeter')).toBeInTheDocument()
    expect(screen.getByText('Says hi')).toBeInTheDocument()
  })

  it('toggles visibility via POST when make-private is clicked', async () => {
    renderPage()
    await screen.findByRole('heading', { name: 'Example Agent' })
    fireEvent.click(screen.getByRole('button', { name: /make private/i }))
    await waitFor(() => {
      expect(mockPOST).toHaveBeenCalledWith(
        '/api/v1/agents/{namespace}/{slug}/visibility',
        {
          params: { path: { namespace: 'acme', slug: 'example-agent' } },
          body: { visibility: 'private' },
        },
      )
    })
  })

  it('submits a PATCH when the edit form is saved', async () => {
    renderPage()
    await screen.findByRole('heading', { name: 'Example Agent' })
    fireEvent.click(screen.getByRole('button', { name: /^edit$/i }))
    fireEvent.click(screen.getByRole('button', { name: /save changes/i }))
    await waitFor(() => {
      expect(mockPATCH).toHaveBeenCalledWith('/api/v1/agents/{namespace}/{slug}', {
        params: { path: { namespace: 'acme', slug: 'example-agent' } },
        body: {
          name: 'Example Agent',
          description: 'An example agent',
        },
      })
    })
  })

  it('shows a not-found state when the query errors', async () => {
    mockGET.mockRejectedValueOnce(new Error('nope'))
    renderPage()
    expect(await screen.findByText(/not found/i)).toBeInTheDocument()
  })
})
