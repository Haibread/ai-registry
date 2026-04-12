/**
 * filter-bar.test.tsx
 *
 * Unit tests for the FilterBar client component.
 *
 * react-router-dom is mocked so the component can render in jsdom without a
 * real router. Because the component initialises its local state from
 * useLocation (the URL is the source of truth), tests set
 * mockSearchParamsString before rendering to simulate active filters.
 *
 * We verify:
 *  - Text inputs are pre-filled from the active URL params.
 *  - Select dropdowns render all provided options and reflect the URL param.
 *  - Visibility filter is hidden by default; shown when showVisibility=true.
 *  - Clear button appears only when at least one filter is active, and calls
 *    navigate(pathname) when clicked.
 *  - Typing in a text input calls navigate() only after the debounce.
 *  - Changing a select calls navigate() immediately.
 *  - Cursor param is always stripped when any filter changes.
 */

import { render, screen, fireEvent, act } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { FilterBar } from './filter-bar'

// ── Mock react-router-dom ─────────────────────────────────────────────────────

const mockNavigate = vi.fn()
const PATHNAME = '/mcp'
let mockSearchParamsString = ''

vi.mock('react-router-dom', async (importOriginal) => {
  const actual = await importOriginal<typeof import('react-router-dom')>()
  return {
    ...actual,
    useNavigate: () => mockNavigate,
    useLocation: () => ({
      pathname: PATHNAME,
      search: mockSearchParamsString ? '?' + mockSearchParamsString : '',
      hash: '',
      state: null,
      key: 'default',
    }),
    useSearchParams: () => [new URLSearchParams(mockSearchParamsString), vi.fn()],
  }
})

beforeEach(() => {
  mockNavigate.mockClear()
  mockSearchParamsString = ''
  vi.useFakeTimers()
})

// ── Helpers ───────────────────────────────────────────────────────────────────

function typeInto(input: HTMLElement, value: string) {
  fireEvent.change(input, { target: { value } })
}

function flushDebounce() {
  act(() => { vi.advanceTimersByTime(350) })
}

// ── Rendering ─────────────────────────────────────────────────────────────────

describe('FilterBar — rendering', () => {
  it('pre-fills search input from URL param q', () => {
    mockSearchParamsString = 'q=hello'
    render(<FilterBar statusOptions={[]} />)
    expect((screen.getByPlaceholderText('Search…') as HTMLInputElement).value).toBe('hello')
  })

  it('uses a custom searchPlaceholder', () => {
    render(<FilterBar searchPlaceholder="Search servers…" statusOptions={[]} />)
    expect(screen.getByPlaceholderText('Search servers…')).toBeInTheDocument()
  })

  it('pre-fills namespace input from URL param namespace', () => {
    mockSearchParamsString = 'namespace=acme'
    render(<FilterBar statusOptions={[]} />)
    expect((screen.getByPlaceholderText('Publisher…') as HTMLInputElement).value).toBe('acme')
  })

  it('renders status options with title-cased labels', () => {
    render(<FilterBar statusOptions={['draft', 'published', 'deprecated']} />)
    expect(screen.getByRole('option', { name: 'All statuses' })).toBeInTheDocument()
    expect(screen.getByRole('option', { name: 'Draft' })).toBeInTheDocument()
    expect(screen.getByRole('option', { name: 'Published' })).toBeInTheDocument()
    expect(screen.getByRole('option', { name: 'Deprecated' })).toBeInTheDocument()
  })

  it('pre-selects status from URL param', () => {
    mockSearchParamsString = 'status=published'
    render(<FilterBar statusOptions={['draft', 'published', 'deprecated']} />)
    expect((screen.getByLabelText('Filter by status') as HTMLSelectElement).value).toBe('published')
  })

  it('hides the visibility filter by default', () => {
    render(<FilterBar statusOptions={[]} />)
    expect(screen.queryByLabelText('Filter by visibility')).not.toBeInTheDocument()
  })

  it('shows the visibility filter when showVisibility=true', () => {
    render(<FilterBar statusOptions={[]} showVisibility />)
    expect(screen.getByLabelText('Filter by visibility')).toBeInTheDocument()
    expect(screen.getByRole('option', { name: 'All visibility' })).toBeInTheDocument()
    expect(screen.getByRole('option', { name: 'Public' })).toBeInTheDocument()
    expect(screen.getByRole('option', { name: 'Private' })).toBeInTheDocument()
  })

  it('pre-selects visibility from URL param', () => {
    mockSearchParamsString = 'visibility=private'
    render(<FilterBar statusOptions={[]} showVisibility />)
    expect((screen.getByLabelText('Filter by visibility') as HTMLSelectElement).value).toBe('private')
  })

  it('renders inside a <form> element', () => {
    const { container } = render(<FilterBar statusOptions={[]} />)
    expect(container.querySelector('form')).toBeTruthy()
  })
})

// ── Clear button ──────────────────────────────────────────────────────────────

describe('FilterBar — Clear button', () => {
  it('is always visible but disabled when no filters are active', () => {
    render(<FilterBar statusOptions={[]} />)
    const btn = screen.getByRole('button', { name: /clear/i })
    expect(btn).toBeInTheDocument()
    expect(btn).toBeDisabled()
  })

  it('is shown when URL param q is set', () => {
    mockSearchParamsString = 'q=hello'
    render(<FilterBar statusOptions={[]} />)
    expect(screen.getByRole('button', { name: /clear/i })).toBeInTheDocument()
  })

  it('is shown when URL param namespace is set', () => {
    mockSearchParamsString = 'namespace=acme'
    render(<FilterBar statusOptions={[]} />)
    expect(screen.getByRole('button', { name: /clear/i })).toBeInTheDocument()
  })

  it('is shown when URL param status is set', () => {
    mockSearchParamsString = 'status=draft'
    render(<FilterBar statusOptions={['draft']} />)
    expect(screen.getByRole('button', { name: /clear/i })).toBeInTheDocument()
  })

  it('is shown when URL param visibility is set', () => {
    mockSearchParamsString = 'visibility=public'
    render(<FilterBar statusOptions={[]} showVisibility />)
    expect(screen.getByRole('button', { name: /clear/i })).toBeInTheDocument()
  })

  it('calls navigate(pathname) with no params when clicked', () => {
    mockSearchParamsString = 'q=test'
    render(<FilterBar statusOptions={[]} />)
    fireEvent.click(screen.getByRole('button', { name: /clear/i }))
    expect(mockNavigate).toHaveBeenCalledWith(PATHNAME, { replace: true })
  })
})

// ── Text input debounce ───────────────────────────────────────────────────────

describe('FilterBar — text input debounce', () => {
  it('does NOT call navigate immediately on typing', () => {
    render(<FilterBar statusOptions={[]} />)
    typeInto(screen.getByPlaceholderText('Search…'), 'foo')
    expect(mockNavigate).not.toHaveBeenCalled()
  })

  it('calls navigate after the debounce window', () => {
    render(<FilterBar statusOptions={[]} />)
    typeInto(screen.getByPlaceholderText('Search…'), 'foo')
    flushDebounce()
    expect(mockNavigate).toHaveBeenCalledOnce()
    expect(mockNavigate.mock.calls[0][0]).toContain('q=foo')
  })

  it('debounces multiple keystrokes into a single navigation', () => {
    render(<FilterBar statusOptions={[]} />)
    const input = screen.getByPlaceholderText('Search…')
    typeInto(input, 'f')
    act(() => { vi.advanceTimersByTime(100) })
    typeInto(input, 'fo')
    act(() => { vi.advanceTimersByTime(100) })
    typeInto(input, 'foo')
    flushDebounce()
    // Only the final value triggers a navigation.
    expect(mockNavigate).toHaveBeenCalledOnce()
    expect(mockNavigate.mock.calls[0][0]).toContain('q=foo')
  })

  it('removes q param from URL when input is cleared', () => {
    mockSearchParamsString = 'q=hello'
    render(<FilterBar statusOptions={[]} />)
    typeInto(screen.getByPlaceholderText('Search…'), '')
    flushDebounce()
    expect(mockNavigate).toHaveBeenCalledOnce()
    expect(mockNavigate.mock.calls[0][0]).not.toContain('q=')
  })

  it('namespace input also debounces', () => {
    render(<FilterBar statusOptions={[]} />)
    typeInto(screen.getByPlaceholderText('Publisher…'), 'acme')
    expect(mockNavigate).not.toHaveBeenCalled()
    flushDebounce()
    expect(mockNavigate).toHaveBeenCalledOnce()
    expect(mockNavigate.mock.calls[0][0]).toContain('namespace=acme')
  })
})

// ── Select immediate navigation ───────────────────────────────────────────────

describe('FilterBar — select immediate navigation', () => {
  it('calls navigate immediately when status changes', () => {
    render(<FilterBar statusOptions={['draft', 'published']} />)
    fireEvent.change(screen.getByLabelText('Filter by status'), {
      target: { value: 'published' },
    })
    expect(mockNavigate).toHaveBeenCalledOnce()
    expect(mockNavigate.mock.calls[0][0]).toContain('status=published')
  })

  it('removes status param when "All statuses" is selected', () => {
    mockSearchParamsString = 'status=draft'
    render(<FilterBar statusOptions={['draft', 'published']} />)
    fireEvent.change(screen.getByLabelText('Filter by status'), {
      target: { value: '' },
    })
    expect(mockNavigate).toHaveBeenCalledOnce()
    expect(mockNavigate.mock.calls[0][0]).not.toContain('status=')
  })

  it('calls navigate immediately when visibility changes', () => {
    render(<FilterBar statusOptions={[]} showVisibility />)
    fireEvent.change(screen.getByLabelText('Filter by visibility'), {
      target: { value: 'private' },
    })
    expect(mockNavigate).toHaveBeenCalledOnce()
    expect(mockNavigate.mock.calls[0][0]).toContain('visibility=private')
  })

  it('strips cursor param when a filter changes', () => {
    mockSearchParamsString = 'cursor=abc123'
    render(<FilterBar statusOptions={['draft']} />)
    fireEvent.change(screen.getByLabelText('Filter by status'), {
      target: { value: 'draft' },
    })
    expect(mockNavigate.mock.calls[0][0]).not.toContain('cursor=')
  })
})

// ── Transport filter ─────────────────────────────────────────────────────────

describe('FilterBar — transport filter', () => {
  it('renders transport select when transportOptions are provided', () => {
    render(<FilterBar statusOptions={[]} transportOptions={['stdio', 'sse', 'streamable_http']} />)
    expect(screen.getByLabelText('Filter by transport')).toBeInTheDocument()
    expect(screen.getByRole('option', { name: 'All transports' })).toBeInTheDocument()
    expect(screen.getByRole('option', { name: 'stdio' })).toBeInTheDocument()
    expect(screen.getByRole('option', { name: 'sse' })).toBeInTheDocument()
    expect(screen.getByRole('option', { name: 'streamable_http' })).toBeInTheDocument()
  })

  it('does not render transport select when transportOptions is empty', () => {
    render(<FilterBar statusOptions={[]} />)
    expect(screen.queryByLabelText('Filter by transport')).not.toBeInTheDocument()
  })

  it('calls navigate immediately when transport changes', () => {
    render(<FilterBar statusOptions={[]} transportOptions={['stdio', 'sse']} />)
    fireEvent.change(screen.getByLabelText('Filter by transport'), {
      target: { value: 'stdio' },
    })
    expect(mockNavigate).toHaveBeenCalledOnce()
    expect(mockNavigate.mock.calls[0][0]).toContain('transport=stdio')
  })

  it('pre-selects transport from URL param', () => {
    mockSearchParamsString = 'transport=sse'
    render(<FilterBar statusOptions={[]} transportOptions={['stdio', 'sse']} />)
    expect((screen.getByLabelText('Filter by transport') as HTMLSelectElement).value).toBe('sse')
  })
})

// ── Registry type / ecosystem filter ─────────────────────────────────────────

describe('FilterBar — registry type filter', () => {
  it('is hidden when transport is not stdio', () => {
    render(<FilterBar statusOptions={[]} transportOptions={['stdio', 'sse']} registryTypeOptions={['npm', 'pypi']} />)
    expect(screen.queryByLabelText('Filter by ecosystem')).not.toBeInTheDocument()
  })

  it('is shown when transport is stdio', () => {
    mockSearchParamsString = 'transport=stdio'
    render(<FilterBar statusOptions={[]} transportOptions={['stdio', 'sse']} registryTypeOptions={['npm', 'pypi']} />)
    expect(screen.getByLabelText('Filter by ecosystem')).toBeInTheDocument()
    expect(screen.getByRole('option', { name: 'All ecosystems' })).toBeInTheDocument()
    expect(screen.getByRole('option', { name: 'npm' })).toBeInTheDocument()
    expect(screen.getByRole('option', { name: 'pypi' })).toBeInTheDocument()
  })

  it('calls navigate immediately when registry type changes', () => {
    mockSearchParamsString = 'transport=stdio'
    render(<FilterBar statusOptions={[]} transportOptions={['stdio']} registryTypeOptions={['npm', 'pypi']} />)
    fireEvent.change(screen.getByLabelText('Filter by ecosystem'), {
      target: { value: 'npm' },
    })
    expect(mockNavigate).toHaveBeenCalledOnce()
    expect(mockNavigate.mock.calls[0][0]).toContain('registry_type=npm')
  })
})

// ── Sort filter ──────────────────────────────────────────────────────────────

describe('FilterBar — sort filter', () => {
  const sortOptions = [
    { value: '', label: 'Newest first' },
    { value: 'updated_at_desc', label: 'Recently updated' },
    { value: 'name_asc', label: 'Name A–Z' },
    { value: 'name_desc', label: 'Name Z–A' },
  ]

  it('renders sort select when sortOptions are provided', () => {
    render(<FilterBar statusOptions={[]} sortOptions={sortOptions} />)
    expect(screen.getByLabelText('Sort by')).toBeInTheDocument()
    expect(screen.getByRole('option', { name: 'Newest first' })).toBeInTheDocument()
    expect(screen.getByRole('option', { name: 'Recently updated' })).toBeInTheDocument()
    expect(screen.getByRole('option', { name: 'Name A–Z' })).toBeInTheDocument()
    expect(screen.getByRole('option', { name: 'Name Z–A' })).toBeInTheDocument()
  })

  it('does not render sort select when sortOptions is empty', () => {
    render(<FilterBar statusOptions={[]} />)
    expect(screen.queryByLabelText('Sort by')).not.toBeInTheDocument()
  })

  it('calls navigate immediately when sort changes', () => {
    render(<FilterBar statusOptions={[]} sortOptions={sortOptions} />)
    fireEvent.change(screen.getByLabelText('Sort by'), {
      target: { value: 'name_asc' },
    })
    expect(mockNavigate).toHaveBeenCalledOnce()
    expect(mockNavigate.mock.calls[0][0]).toContain('sort=name_asc')
  })

  it('pre-selects sort from URL param', () => {
    mockSearchParamsString = 'sort=name_desc'
    render(<FilterBar statusOptions={[]} sortOptions={sortOptions} />)
    expect((screen.getByLabelText('Sort by') as HTMLSelectElement).value).toBe('name_desc')
  })

  it('removes sort param when default option is selected', () => {
    mockSearchParamsString = 'sort=name_asc'
    render(<FilterBar statusOptions={[]} sortOptions={sortOptions} />)
    fireEvent.change(screen.getByLabelText('Sort by'), {
      target: { value: '' },
    })
    expect(mockNavigate).toHaveBeenCalledOnce()
    expect(mockNavigate.mock.calls[0][0]).not.toContain('sort=')
  })

  it('includes sort in hasFilters check — Clear is enabled when sort is set', () => {
    mockSearchParamsString = 'sort=name_asc'
    render(<FilterBar statusOptions={[]} sortOptions={sortOptions} />)
    const btn = screen.getByRole('button', { name: /clear/i })
    expect(btn).not.toBeDisabled()
  })
})
