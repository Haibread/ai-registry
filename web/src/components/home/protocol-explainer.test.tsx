import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { ProtocolExplainer } from './protocol-explainer'

describe('ProtocolExplainer', () => {
  it('renders the toggle button', () => {
    render(<ProtocolExplainer />)
    expect(screen.getByRole('button', { name: /what are mcp and a2a/i })).toBeInTheDocument()
  })

  it('starts collapsed — descriptions are not visible', () => {
    render(<ProtocolExplainer />)
    expect(screen.queryByText(/model context protocol/i)).not.toBeInTheDocument()
    expect(screen.queryByText(/agent-to-agent protocol/i)).not.toBeInTheDocument()
  })

  it('expands on click to show MCP description', async () => {
    const user = userEvent.setup()
    render(<ProtocolExplainer />)
    await user.click(screen.getByRole('button'))
    expect(screen.getByText(/model context protocol/i)).toBeInTheDocument()
    expect(screen.getByText(/open standard for connecting ai models/i)).toBeInTheDocument()
  })

  it('expands on click to show A2A description', async () => {
    const user = userEvent.setup()
    render(<ProtocolExplainer />)
    await user.click(screen.getByRole('button'))
    expect(screen.getByText(/agent-to-agent protocol/i)).toBeInTheDocument()
    expect(screen.getByText(/protocol for ai agents to discover/i)).toBeInTheDocument()
  })

  it('includes external links to spec sites', async () => {
    const user = userEvent.setup()
    render(<ProtocolExplainer />)
    await user.click(screen.getByRole('button'))

    const mcpLink = screen.getByRole('link', { name: /modelcontextprotocol\.io/i })
    expect(mcpLink).toHaveAttribute('href', 'https://modelcontextprotocol.io/')
    expect(mcpLink).toHaveAttribute('target', '_blank')

    const a2aLink = screen.getByRole('link', { name: /a2a-protocol\.org/i })
    expect(a2aLink).toHaveAttribute('href', 'https://a2a-protocol.org/')
    expect(a2aLink).toHaveAttribute('target', '_blank')
  })

  it('collapses again on second click', async () => {
    const user = userEvent.setup()
    render(<ProtocolExplainer />)
    const btn = screen.getByRole('button')
    await user.click(btn)
    expect(screen.getByText(/model context protocol/i)).toBeInTheDocument()
    await user.click(btn)
    expect(screen.queryByText(/model context protocol/i)).not.toBeInTheDocument()
  })

  it('sets aria-expanded correctly', async () => {
    const user = userEvent.setup()
    render(<ProtocolExplainer />)
    const btn = screen.getByRole('button')
    expect(btn).toHaveAttribute('aria-expanded', 'false')
    await user.click(btn)
    expect(btn).toHaveAttribute('aria-expanded', 'true')
  })
})
