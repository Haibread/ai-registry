import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import type { ReactNode } from 'react'
import { PublisherProvider, usePublisher } from './PublisherContext'
import type { Role } from './useMe'

interface MockGrant {
  role: Role
  publisher_slug?: string
  publisher_name?: string
}
interface MockPerms {
  grants: MockGrant[]
  isServerAdmin: boolean
  isLoading: boolean
}

let mockPerms: MockPerms
vi.mock('@/auth/useMe', () => ({
  usePermissions: () => mockPerms,
}))

vi.mock('@/auth/AuthContext', () => ({ useAuth: () => ({ accessToken: 't' }) }))

const mockGET = vi.fn()
vi.mock('@/lib/api-client', () => ({ useAuthClient: () => ({ GET: mockGET }) }))

function Probe() {
  const { publishers, currentSlug, current, setCurrent } = usePublisher()
  return (
    <div>
      <span data-testid="current">{currentSlug ?? 'ALL'}</span>
      <span data-testid="current-name">{current?.name ?? '-'}</span>
      <span data-testid="count">{publishers.length}</span>
      <span data-testid="roles">
        {publishers.map((p) => `${p.slug}:${p.roles.join('+')}`).join(',')}
      </span>
      <button onClick={() => setCurrent('globex')}>pick-globex</button>
      <button onClick={() => setCurrent(null)}>pick-all</button>
    </div>
  )
}

function renderProbe() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={qc}>
      <PublisherProvider>{children}</PublisherProvider>
    </QueryClientProvider>
  )
  return render(<Probe />, { wrapper })
}

beforeEach(() => {
  localStorage.clear()
  vi.clearAllMocks()
  mockPerms = { grants: [], isServerAdmin: false, isLoading: false }
  mockGET.mockResolvedValue({ data: { items: [] } })
})

describe('PublisherProvider', () => {
  it('derives distinct publishers from grants, unions roles, and excludes global grants', () => {
    mockPerms = {
      isServerAdmin: false,
      isLoading: false,
      grants: [
        { role: 'editor', publisher_slug: 'acme', publisher_name: 'Acme' },
        { role: 'reviewer', publisher_slug: 'acme', publisher_name: 'Acme' },
        { role: 'viewer', publisher_slug: 'globex', publisher_name: 'Globex' },
        { role: 'admin' }, // global grant — excluded from the switcher
      ],
    }
    renderProbe()
    expect(screen.getByTestId('count').textContent).toBe('2')
    expect(screen.getByTestId('roles').textContent).toBe('acme:editor+reviewer,globex:viewer')
    expect(screen.getByTestId('current').textContent).toBe('acme')
    expect(screen.getByTestId('current-name').textContent).toBe('Acme')
  })

  it('persists an explicit switch to localStorage', () => {
    mockPerms = {
      isServerAdmin: false,
      isLoading: false,
      grants: [
        { role: 'editor', publisher_slug: 'acme', publisher_name: 'Acme' },
        { role: 'editor', publisher_slug: 'globex', publisher_name: 'Globex' },
      ],
    }
    renderProbe()
    fireEvent.click(screen.getByText('pick-globex'))
    expect(screen.getByTestId('current').textContent).toBe('globex')
    expect(localStorage.getItem('ai-registry.current_publisher')).toBe('globex')
  })

  it('defaults a Server Admin to the All-publishers (global) view', () => {
    mockPerms = {
      isServerAdmin: true,
      isLoading: false,
      grants: [{ role: 'editor', publisher_slug: 'acme', publisher_name: 'Acme' }],
    }
    renderProbe()
    // A Server Admin lands on All (global dashboard), not snapped to a publisher.
    expect(screen.getByTestId('current').textContent).toBe('ALL')
  })

  it('persists a Server Admin pick of a specific publisher and the All choice', () => {
    localStorage.setItem('ai-registry.current_publisher', 'globex')
    mockPerms = {
      isServerAdmin: true,
      isLoading: false,
      grants: [
        { role: 'editor', publisher_slug: 'acme', publisher_name: 'Acme' },
        { role: 'editor', publisher_slug: 'globex', publisher_name: 'Globex' },
      ],
    }
    renderProbe()
    // A stored, still-valid publisher selection is honored over the All default.
    expect(screen.getByTestId('current').textContent).toBe('globex')
    fireEvent.click(screen.getByText('pick-all'))
    expect(screen.getByTestId('current').textContent).toBe('ALL')
    expect(localStorage.getItem('ai-registry.current_publisher')).toBe('__all__')
  })

  it('honors a still-valid stored slug over the default', () => {
    localStorage.setItem('ai-registry.current_publisher', 'globex')
    mockPerms = {
      isServerAdmin: false,
      isLoading: false,
      grants: [
        { role: 'editor', publisher_slug: 'acme', publisher_name: 'Acme' },
        { role: 'editor', publisher_slug: 'globex', publisher_name: 'Globex' },
      ],
    }
    renderProbe()
    expect(screen.getByTestId('current').textContent).toBe('globex')
  })

  it('offers a Server Admin every publisher, not just the granted ones', async () => {
    mockPerms = {
      isServerAdmin: true,
      isLoading: false,
      grants: [{ role: 'editor', publisher_slug: 'acme', publisher_name: 'Acme' }],
    }
    mockGET.mockResolvedValue({
      data: { items: [{ slug: 'globex', name: 'Globex' }, { slug: 'acme', name: 'Acme' }] },
    })
    renderProbe()
    // Once the publisher list resolves, the ungranted publisher (globex) is
    // folded in with no roles, alongside the granted one.
    await waitFor(() => expect(screen.getByTestId('count').textContent).toBe('2'))
    expect(screen.getByTestId('roles').textContent).toBe('acme:editor,globex:')
  })
})
