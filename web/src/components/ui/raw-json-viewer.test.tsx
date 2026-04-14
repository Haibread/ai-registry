import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { RawJsonViewer } from './raw-json-viewer'

describe('RawJsonViewer', () => {
  const writeText = vi.fn().mockResolvedValue(undefined)

  beforeEach(() => {
    writeText.mockClear()
    Object.assign(navigator, { clipboard: { writeText } })
  })

  it('renders the title collapsed by default', () => {
    render(<RawJsonViewer data={{ a: 1 }} />)
    expect(screen.getByText('Raw JSON')).toBeInTheDocument()
    expect(screen.queryByText(/"a": 1/)).not.toBeInTheDocument()
  })

  it('shows custom title', () => {
    render(<RawJsonViewer data={{ a: 1 }} title="Server JSON" />)
    expect(screen.getByText('Server JSON')).toBeInTheDocument()
  })

  it('expands and renders JSON when the header is clicked', () => {
    render(<RawJsonViewer data={{ a: 1 }} />)
    fireEvent.click(screen.getByText('Raw JSON'))
    expect(screen.getByText(/"a": 1/)).toBeInTheDocument()
  })

  it('renders open when defaultOpen is true and shows Copy button', () => {
    render(<RawJsonViewer data={{ hello: 'world' }} defaultOpen />)
    expect(screen.getByText(/"hello": "world"/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /copy json to clipboard/i })).toBeInTheDocument()
  })

  it('copies JSON to clipboard and flips to Copied state', async () => {
    render(<RawJsonViewer data={{ hello: 'world' }} defaultOpen />)
    fireEvent.click(screen.getByRole('button', { name: /copy json to clipboard/i }))
    await waitFor(() => {
      expect(writeText).toHaveBeenCalledWith(JSON.stringify({ hello: 'world' }, null, 2))
    })
    expect(await screen.findByText(/^Copied$/)).toBeInTheDocument()
  })
})
