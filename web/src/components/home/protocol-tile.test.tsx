import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { ProtocolTile } from './protocol-tile'

function renderTile(props: Parameters<typeof ProtocolTile>[0] = {}) {
  return render(
    <MemoryRouter>
      <ProtocolTile {...props} />
    </MemoryRouter>,
  )
}

describe('ProtocolTile', () => {
  it('explains the protocol', () => {
    renderTile()
    expect(screen.getByRole('heading', { name: 'MCP servers' })).toBeInTheDocument()
    expect(screen.getByText(/model context protocol gives ai models tools/i)).toBeInTheDocument()
  })

  it('links to the catalog with the entry count', () => {
    renderTile({ mcpCount: 12 })
    expect(screen.getByRole('link', { name: /browse 12 servers/i })).toHaveAttribute('href', '/mcp')
  })

  it('uses the singular for a single entry', () => {
    renderTile({ mcpCount: 1 })
    expect(screen.getByRole('link', { name: /browse 1 server$/i })).toHaveAttribute('href', '/mcp')
  })

  it('omits the count until stats load', () => {
    renderTile()
    expect(screen.getByRole('link', { name: /browse servers/i })).toHaveAttribute('href', '/mcp')
  })

  it('links to the protocol spec in a new tab', () => {
    renderTile()
    const mcp = screen.getByRole('link', { name: 'What is MCP?' })
    expect(mcp).toHaveAttribute('href', 'https://modelcontextprotocol.io/')
    expect(mcp).toHaveAttribute('target', '_blank')
  })
})
