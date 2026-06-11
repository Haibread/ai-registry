import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'

const { mockToast } = vi.hoisted(() => ({ mockToast: { success: vi.fn(), error: vi.fn() } }))
vi.mock('sonner', () => ({ toast: mockToast }))

const mockGET = vi.fn()
const mockPOST = vi.fn()
const mockPATCH = vi.fn()
const mockDELETE = vi.fn()
vi.mock('@/lib/api-client', () => ({
  useAuthClient: () => ({ GET: mockGET, POST: mockPOST, PATCH: mockPATCH, DELETE: mockDELETE }),
}))

import AdminTagList from './tags'

// JSDOM doesn't implement HTMLDialogElement's modal methods; stub them so the
// ConfirmDialog opens and closes.
beforeEach(() => {
  if (!HTMLDialogElement.prototype.showModal) {
    HTMLDialogElement.prototype.showModal = function () {
      this.setAttribute('open', '')
    }
  }
  if (!HTMLDialogElement.prototype.close) {
    HTMLDialogElement.prototype.close = function () {
      this.removeAttribute('open')
      this.dispatchEvent(new Event('close'))
    }
  }
})

const earlyAccess = {
  slug: 'early-access',
  name: 'Early Access',
  description: 'Pre-GA functionality',
  color: 'blue',
  active: true,
  created_at: '2026-06-01T10:00:00Z',
  updated_at: '2026-06-01T10:00:00Z',
}
const legacy = {
  slug: 'legacy',
  name: 'Legacy',
  color: 'gray',
  active: false,
  created_at: '2026-06-02T10:00:00Z',
  updated_at: '2026-06-02T10:00:00Z',
}

function renderPage() {
  const qc = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  return render(
    <QueryClientProvider client={qc}>
      <AdminTagList />
    </QueryClientProvider>,
  )
}

/** The table row carrying the given tag name. */
function rowOf(name: string) {
  const row = screen.getByText(name).closest('tr')
  expect(row).not.toBeNull()
  return within(row!)
}

describe('AdminTagList', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockGET.mockResolvedValue({ data: { items: [earlyAccess, legacy] } })
  })

  it('renders the vocabulary with active and deactivated tags', async () => {
    renderPage()
    expect(screen.getByRole('heading', { name: /instance tags/i })).toBeInTheDocument()
    expect(await screen.findByText('Early Access')).toBeInTheDocument()
    expect(mockGET).toHaveBeenCalledWith('/api/v1/tags')
    expect(rowOf('Early Access').getByText('Active')).toBeInTheDocument()
    expect(rowOf('Legacy').getByText('Deactivated')).toBeInTheDocument()
  })

  it('shows the empty state when no tags are defined', async () => {
    mockGET.mockResolvedValueOnce({ data: { items: [] } })
    renderPage()
    expect(await screen.findByText(/no tags yet/i)).toBeInTheDocument()
  })

  it('creates a tag from the form via POST', async () => {
    mockPOST.mockResolvedValue({
      data: { ...earlyAccess, slug: 'beta', name: 'Beta', description: 'Not GA yet', color: 'orange' },
    })
    renderPage()
    await screen.findByText('Early Access')

    await userEvent.type(screen.getByLabelText(/^slug$/i), 'beta')
    await userEvent.type(screen.getByLabelText(/display name/i), 'Beta')
    await userEvent.type(screen.getByLabelText(/^description$/i), 'Not GA yet')
    await userEvent.selectOptions(screen.getByLabelText(/^color$/i), 'orange')
    await userEvent.click(screen.getByRole('button', { name: /add tag/i }))

    await waitFor(() => {
      expect(mockPOST).toHaveBeenCalledWith('/api/v1/tags', {
        body: { slug: 'beta', name: 'Beta', description: 'Not GA yet', color: 'orange' },
      })
    })
    expect(mockToast.success).toHaveBeenCalledWith('Tag "Beta" created')
    // The form resets for the next entry.
    expect(screen.getByLabelText(/^slug$/i)).toHaveValue('')
  })

  it('omits an empty description from the create request', async () => {
    mockPOST.mockResolvedValue({ data: { ...earlyAccess, slug: 'beta', name: 'Beta' } })
    renderPage()
    await screen.findByText('Early Access')

    await userEvent.type(screen.getByLabelText(/^slug$/i), 'beta')
    await userEvent.type(screen.getByLabelText(/display name/i), 'Beta')
    await userEvent.click(screen.getByRole('button', { name: /add tag/i }))

    await waitFor(() => {
      expect(mockPOST).toHaveBeenCalledWith('/api/v1/tags', {
        body: { slug: 'beta', name: 'Beta', description: undefined, color: 'gray' },
      })
    })
  })

  it('deactivates an active tag via PATCH', async () => {
    mockPATCH.mockResolvedValue({ data: { ...earlyAccess, active: false } })
    renderPage()
    await screen.findByText('Early Access')

    await userEvent.click(rowOf('Early Access').getByRole('button', { name: /^deactivate$/i }))

    await waitFor(() => {
      expect(mockPATCH).toHaveBeenCalledWith('/api/v1/tags/{slug}', {
        params: { path: { slug: 'early-access' } },
        body: { active: false },
      })
    })
    expect(mockToast.success).toHaveBeenCalledWith('Tag "Early Access" updated')
  })

  it('reactivates a deactivated tag via PATCH', async () => {
    mockPATCH.mockResolvedValue({ data: { ...legacy, active: true } })
    renderPage()
    await screen.findByText('Legacy')

    await userEvent.click(rowOf('Legacy').getByRole('button', { name: /^activate$/i }))

    await waitFor(() => {
      expect(mockPATCH).toHaveBeenCalledWith('/api/v1/tags/{slug}', {
        params: { path: { slug: 'legacy' } },
        body: { active: true },
      })
    })
  })

  it('deletes a tag only after confirmation', async () => {
    mockDELETE.mockResolvedValue({ error: undefined })
    renderPage()
    await screen.findByText('Early Access')

    await userEvent.click(rowOf('Early Access').getByRole('button', { name: /^delete$/i }))
    // One click must not mutate — the confirmation names the tag first.
    expect(mockDELETE).not.toHaveBeenCalled()

    const dialog = screen
      .getByRole('heading', { name: /delete tag "early access"\?/i })
      .closest('dialog')!
    await userEvent.click(within(dialog).getByRole('button', { name: /^delete$/i }))

    await waitFor(() => {
      expect(mockDELETE).toHaveBeenCalledWith('/api/v1/tags/{slug}', {
        params: { path: { slug: 'early-access' } },
      })
    })
    expect(mockToast.success).toHaveBeenCalledWith('Tag deleted')
  })

  it('cancelling the delete confirmation leaves the tag untouched', async () => {
    renderPage()
    await screen.findByText('Early Access')

    await userEvent.click(rowOf('Early Access').getByRole('button', { name: /^delete$/i }))
    await userEvent.click(screen.getByRole('button', { name: /^cancel$/i }))
    expect(mockDELETE).not.toHaveBeenCalled()
  })

  it('surfaces the 409 problem detail when deleting an in-use tag', async () => {
    mockDELETE.mockResolvedValue({
      error: {
        title: 'tag-in-use',
        detail: 'The tag is carried by a published version; deactivate it instead.',
      },
    })
    renderPage()
    await screen.findByText('Early Access')

    await userEvent.click(rowOf('Early Access').getByRole('button', { name: /^delete$/i }))
    const dialog = screen
      .getByRole('heading', { name: /delete tag "early access"\?/i })
      .closest('dialog')!
    await userEvent.click(within(dialog).getByRole('button', { name: /^delete$/i }))

    await waitFor(() => {
      expect(mockToast.error).toHaveBeenCalledWith(
        'The tag is carried by a published version; deactivate it instead.',
      )
    })
    expect(mockToast.success).not.toHaveBeenCalled()
  })

  it('updates display fields through the edit dialog via PATCH', async () => {
    mockPATCH.mockResolvedValue({ data: { ...earlyAccess, name: 'GA Preview' } })
    renderPage()
    await screen.findByText('Early Access')

    await userEvent.click(rowOf('Early Access').getByRole('button', { name: /^edit$/i }))
    const nameInput = screen.getByLabelText(/^display name$/i, {
      selector: '#edit-tag-name',
    })
    await userEvent.clear(nameInput)
    await userEvent.type(nameInput, 'GA Preview')
    await userEvent.click(screen.getByRole('button', { name: /^save$/i }))

    await waitFor(() => {
      expect(mockPATCH).toHaveBeenCalledWith('/api/v1/tags/{slug}', {
        params: { path: { slug: 'early-access' } },
        body: { name: 'GA Preview', description: 'Pre-GA functionality', color: 'blue' },
      })
    })
  })

  it('shows the error state when the vocabulary fails to load', async () => {
    mockGET.mockResolvedValueOnce({ data: undefined })
    renderPage()
    expect(await screen.findByText(/failed to load tags/i)).toBeInTheDocument()
  })
})

describe('AdminTagList: config-managed tags', () => {
  const managed = {
    slug: 'free',
    name: 'Free',
    description: 'No cost to use',
    color: 'green',
    active: true,
    managed: true,
    created_at: '2026-06-03T10:00:00Z',
    updated_at: '2026-06-03T10:00:00Z',
  }

  beforeEach(() => {
    vi.clearAllMocks()
    mockGET.mockResolvedValue({ data: { items: [managed, earlyAccess] } })
  })

  it('badges managed tags and disables their actions', async () => {
    renderPage()
    const row = rowOf(await screen.findByText('Free').then(() => 'Free'))

    expect(row.getByText('Managed')).toBeInTheDocument()
    expect(row.getByRole('button', { name: 'Edit' })).toBeDisabled()
    expect(row.getByRole('button', { name: 'Deactivate' })).toBeDisabled()
    expect(row.getByRole('button', { name: 'Delete' })).toBeDisabled()

    // Unmanaged rows keep their actions.
    const other = rowOf('Early Access')
    expect(other.queryByText('Managed')).not.toBeInTheDocument()
    expect(other.getByRole('button', { name: 'Edit' })).toBeEnabled()
  })
})
