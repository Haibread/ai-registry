import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import { PublisherSwitcher } from './publisher-switcher'
import type { PublisherOption } from '@/auth/PublisherContext'

interface Ctx {
  publishers: PublisherOption[]
  isServerAdmin: boolean
  currentSlug: string | null
  current: PublisherOption | null
  setCurrent: (slug: string | null) => void
  isLoading: boolean
}

let mockCtx: Ctx
vi.mock('@/auth/PublisherContext', () => ({
  usePublisher: () => mockCtx,
}))

const acme: PublisherOption = { slug: 'acme', name: 'Acme', roles: ['editor'] }
const globex: PublisherOption = { slug: 'globex', name: 'Globex', roles: ['viewer'] }

beforeEach(() => {
  mockCtx = {
    publishers: [],
    isServerAdmin: false,
    currentSlug: null,
    current: null,
    setCurrent: vi.fn(),
    isLoading: false,
  }
})

describe('PublisherSwitcher', () => {
  it('shows a hint when a non-admin has no publishers', () => {
    render(<PublisherSwitcher />)
    expect(screen.getByText(/no publishers yet/i)).toBeInTheDocument()
    expect(screen.queryByRole('combobox')).not.toBeInTheDocument()
  })

  it('renders a static label (no dropdown) for a single-publisher member', () => {
    mockCtx = { ...mockCtx, publishers: [acme], currentSlug: 'acme', current: acme }
    render(<PublisherSwitcher />)
    expect(screen.getByText('Acme')).toBeInTheDocument()
    expect(screen.queryByRole('combobox')).not.toBeInTheDocument()
  })

  it('renders a dropdown when the member belongs to several publishers', () => {
    mockCtx = { ...mockCtx, publishers: [acme, globex], currentSlug: 'acme', current: acme }
    render(<PublisherSwitcher />)
    expect(screen.getByRole('combobox', { name: /switch publisher/i })).toBeInTheDocument()
  })

  it('renders a dropdown for a Server Admin even with a single publisher (All scope)', () => {
    mockCtx = { ...mockCtx, isServerAdmin: true, publishers: [acme], currentSlug: null, current: null }
    render(<PublisherSwitcher />)
    expect(screen.getByRole('combobox', { name: /switch publisher/i })).toBeInTheDocument()
  })
})
