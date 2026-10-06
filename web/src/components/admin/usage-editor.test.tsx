import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import type { components } from '@/lib/schema'

const { mockToast } = vi.hoisted(() => ({ mockToast: { success: vi.fn(), error: vi.fn() } }))
vi.mock('sonner', () => ({ toast: mockToast }))

const mockPATCH = vi.fn()
vi.mock('@/lib/api-client', () => ({
  useAuthClient: () => ({ PATCH: mockPATCH }),
}))

import { UsageEditor } from '@/components/admin/usage-editor'

type MCPServer = components['schemas']['MCPServer']

const server: MCPServer = {
  id: 'srv-1',
  namespace: 'acme',
  slug: 'github-tools',
  name: 'GitHub Tools',
  visibility: 'public',
  status: 'published',
  featured: false,
  verified: false,
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
  latest_version: {
    version: '1.4.2',
    runtime: 'stdio',
    protocol_versions: ['2025-06-18'],
    packages: [
      { registryType: 'npm', identifier: '@acme/github-tools', version: '1.4.2', transport: { type: 'stdio' } },
    ],
  },
}

function renderEditor(props: Partial<React.ComponentProps<typeof UsageEditor>> = {}) {
  const onSaved = vi.fn()
  const qc = new QueryClient({ defaultOptions: { mutations: { retry: false } } })
  render(
    <QueryClientProvider client={qc}>
      <MemoryRouter>
        <UsageEditor server={server} canEdit isServerAdmin changePending={false} onSaved={onSaved} {...props} />
      </MemoryRouter>
    </QueryClientProvider>,
  )
  return { onSaved }
}

const patchBody = () => mockPATCH.mock.calls[0]?.[1]?.body

describe('UsageEditor', () => {
  beforeEach(() => {
    // jsdom lacks the <dialog> modal API that ConfirmDialog drives.
    HTMLDialogElement.prototype.showModal ??= function (this: HTMLDialogElement) {
      this.setAttribute('open', '')
    }
    HTMLDialogElement.prototype.close ??= function (this: HTMLDialogElement) {
      this.removeAttribute('open')
    }
    vi.clearAllMocks()
    mockPATCH.mockResolvedValue({ error: undefined })
  })

  it('saves custom Markdown through PATCH', async () => {
    const user = userEvent.setup()
    const { onSaved } = renderEditor()
    expect(screen.getByText('Generated')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Customize' }))
    await user.type(screen.getByRole('textbox', { name: 'Usage Markdown' }), 'Hello')
    expect(screen.getByText('Unsaved changes')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => expect(onSaved).toHaveBeenCalled())
    expect(patchBody()).toEqual({ usage_markdown: 'Hello' })
  })

  it('starts from a template of the generated instructions', async () => {
    const user = userEvent.setup()
    renderEditor()
    await user.click(screen.getByRole('button', { name: 'Start from generated' }))
    expect(screen.getByRole('textbox', { name: 'Usage Markdown' })).toHaveValue(
      '## Connect\n\nRun the server locally:\n\n```sh\n{{run_command}}\n```\n',
    )
  })

  it('previews the draft with placeholders substituted', async () => {
    const user = userEvent.setup()
    renderEditor({ server: { ...server, usage_markdown: 'Version {{version}}' } })
    await user.click(screen.getByRole('button', { name: 'Edit' }))
    await user.click(screen.getByRole('button', { name: 'preview' }))
    expect(screen.getByText('Version 1.4.2')).toBeInTheDocument()
    expect(screen.queryByRole('textbox')).not.toBeInTheDocument()
  })

  it('resets to the generated instructions after confirmation', async () => {
    const user = userEvent.setup()
    renderEditor({ server: { ...server, usage_markdown: 'Custom' } })
    await user.click(screen.getByRole('button', { name: 'Reset to generated' }))
    await user.click(screen.getByRole('button', { name: 'Reset' }))
    await waitFor(() => expect(patchBody()).toEqual({ usage_markdown: '' }))
  })

  it('labels the save as a review submission for Editors', async () => {
    const user = userEvent.setup()
    renderEditor({ isServerAdmin: false })
    await user.click(screen.getByRole('button', { name: 'Customize' }))
    await user.type(screen.getByRole('textbox', { name: 'Usage Markdown' }), 'x')
    await user.click(screen.getByRole('button', { name: 'Submit for review' }))
    await waitFor(() => expect(mockToast.success).toHaveBeenCalledWith('Submitted for review'))
  })

  it('blocks Editors while another change awaits review', () => {
    renderEditor({ isServerAdmin: false, changePending: true })
    expect(screen.getByRole('button', { name: 'Customize' })).toBeDisabled()
  })

  it('hides edit actions from viewers', () => {
    renderEditor({ canEdit: false })
    expect(screen.queryByRole('button', { name: 'Customize' })).not.toBeInTheDocument()
  })
})
