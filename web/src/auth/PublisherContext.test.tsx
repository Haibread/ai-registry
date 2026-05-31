import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
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
  return render(
    <PublisherProvider>
      <Probe />
    </PublisherProvider>,
  )
}

beforeEach(() => {
  localStorage.clear()
  mockPerms = { grants: [], isServerAdmin: false, isLoading: false }
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
    // Default selection is the first publisher (alphabetical: Acme).
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

  it('lets a Server Admin pick All publishers and keeps it (no snap-back to first)', () => {
    mockPerms = {
      isServerAdmin: true,
      isLoading: false,
      grants: [{ role: 'editor', publisher_slug: 'acme', publisher_name: 'Acme' }],
    }
    renderProbe()
    expect(screen.getByTestId('current').textContent).toBe('acme') // default
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
})
