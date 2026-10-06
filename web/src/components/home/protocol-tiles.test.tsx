import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { ProtocolTiles } from './protocol-tiles'

function renderTiles(props: Parameters<typeof ProtocolTiles>[0] = {}) {
  return render(
    <MemoryRouter>
      <ProtocolTiles {...props} />
    </MemoryRouter>,
  )
}

describe('ProtocolTiles', () => {
  it('explains both protocols', () => {
    renderTiles()
    expect(screen.getByRole('heading', { name: 'MCP servers' })).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'A2A agents' })).toBeInTheDocument()
    expect(screen.getByText(/model context protocol gives ai models tools/i)).toBeInTheDocument()
    expect(screen.getByText(/agent-to-agent protocol/i)).toBeInTheDocument()
  })

  it('links each tile to its catalog with the entry count', () => {
    renderTiles({ mcpCount: 12, agentCount: 1 })
    expect(screen.getByRole('link', { name: /browse 12 servers/i })).toHaveAttribute('href', '/mcp')
    expect(screen.getByRole('link', { name: /browse 1 agent$/i })).toHaveAttribute('href', '/agents')
  })

  it('omits the count until stats load', () => {
    renderTiles()
    expect(screen.getByRole('link', { name: /browse servers/i })).toHaveAttribute('href', '/mcp')
  })

  it('links to the protocol specs in a new tab', () => {
    renderTiles()
    const mcp = screen.getByRole('link', { name: 'What is MCP?' })
    expect(mcp).toHaveAttribute('href', 'https://modelcontextprotocol.io/')
    expect(mcp).toHaveAttribute('target', '_blank')
    expect(screen.getByRole('link', { name: 'What is A2A?' })).toHaveAttribute('href', 'https://a2a-protocol.org/')
  })
})
