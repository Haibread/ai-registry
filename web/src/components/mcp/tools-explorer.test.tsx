import { describe, it, expect } from 'vitest'
import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes, useLocation } from 'react-router-dom'
import { ToolsExplorer } from './tools-explorer'
import type { MCPTool } from '@/lib/mcp-tools'

const TOOLS: MCPTool[] = [
  {
    name: 'issue_list',
    description: 'List issues',
    annotations: { readOnlyHint: true },
    input_schema: { type: 'object', properties: { repo: { type: 'string' } }, required: ['repo'] },
  },
  {
    name: 'branch_delete',
    description: 'Delete a branch',
    annotations: { title: 'Delete branch', destructiveHint: true, team: 'infra' },
    input_schema: { type: 'object', properties: { branch: { type: 'string' } }, required: ['branch'] },
  },
  { name: 'ping' },
]

function LocationProbe() {
  const { search, hash } = useLocation()
  return <output data-testid="location">{search}{hash}</output>
}

function renderExplorer(url = '/mcp/acme/github#tools') {
  return render(
    <MemoryRouter initialEntries={[url]}>
      <Routes>
        <Route path="/mcp/:ns/:slug" element={<><ToolsExplorer tools={TOOLS} /><LocationProbe /></>} />
      </Routes>
    </MemoryRouter>,
  )
}

const detail = () => screen.getByRole('region', { name: /^Tool / })

describe('ToolsExplorer', () => {
  it('summarises the tools and opens the first one', () => {
    renderExplorer()
    expect(screen.getByText('3 tools · 1 read-only · 1 destructive')).toBeInTheDocument()
    expect(within(detail()).getByRole('heading', { name: 'issue_list' })).toBeInTheDocument()
    expect(within(detail()).getByText('Does not modify its environment.')).toBeInTheDocument()
    expect(within(detail()).getByRole('cell', { name: /repo\s*required/i })).toBeInTheDocument()
  })

  it('selects a tool into the URL and keeps the tab hash', async () => {
    const user = userEvent.setup()
    renderExplorer()
    await user.click(screen.getByRole('button', { name: /branch_delete/ }))
    expect(within(detail()).getByRole('heading', { name: 'branch_delete' })).toBeInTheDocument()
    expect(within(detail()).getByText('Delete branch')).toBeInTheDocument()
    expect(within(detail()).getByText('team: "infra"')).toBeInTheDocument()
    expect(screen.getByTestId('location')).toHaveTextContent('?tool=branch_delete#tools')
    expect(screen.getByRole('button', { name: /branch_delete/ })).toHaveAttribute('aria-current', 'true')
  })

  it('opens the tool named in the URL', () => {
    renderExplorer('/mcp/acme/github?tool=ping#tools')
    expect(within(detail()).getByRole('heading', { name: 'ping' })).toBeInTheDocument()
    expect(within(detail()).getByText('This tool takes no parameters.')).toBeInTheDocument()
  })

  it('goes back to the list', async () => {
    const user = userEvent.setup()
    renderExplorer('/mcp/acme/github?tool=ping#tools')
    await user.click(screen.getByRole('button', { name: /all tools/i }))
    expect(screen.getByTestId('location')).toHaveTextContent(/^#tools$/)
  })

  it('filters by name, title and description', async () => {
    const user = userEvent.setup()
    renderExplorer()
    const search = screen.getByRole('searchbox', { name: 'Search tools' })
    await user.type(search, 'delete branch')
    const list = screen.getByRole('list', { name: 'Tools' })
    expect(within(list).getAllByRole('button').map((b) => b.textContent)).toEqual([
      expect.stringContaining('branch_delete'),
    ])
    await user.clear(search)
    await user.type(search, 'nothing')
    expect(screen.getByText('No tool matches “nothing”.')).toBeInTheDocument()
  })

  it('shows the example call and the raw schema', async () => {
    const user = userEvent.setup()
    renderExplorer()
    await user.click(within(detail()).getByRole('tab', { name: 'Example call' }))
    expect(within(detail()).getByText(/"method": "tools\/call"/)).toBeInTheDocument()
    await user.click(within(detail()).getByRole('tab', { name: 'JSON schema' }))
    expect(within(detail()).getByText(/"required": \[/)).toBeInTheDocument()
  })
})
