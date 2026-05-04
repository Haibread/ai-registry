/**
 * admin-sidebar.test.tsx
 *
 * Tests for the active-route detection logic in AdminSidebar.
 *
 * The sidebar has two matching modes:
 *  - exact=true  (Dashboard): only highlights when pathname === "/admin"
 *  - exact=false (all others): highlights when pathname.startsWith(href)
 */

import { render, screen } from '@testing-library/react'
import { describe, it, expect } from 'vitest'
import { MemoryRouter } from 'react-router-dom'
import { AdminSidebar } from './admin-sidebar'

function renderSidebar(pathname: string) {
  return render(
    <MemoryRouter initialEntries={[pathname]}>
      <AdminSidebar pathname={pathname} />
    </MemoryRouter>
  )
}

function linkClass(label: string): string {
  return screen.getByRole('link', { name: new RegExp(label, 'i') }).className
}

const ACTIVE_CLASS = 'bg-accent text-accent-foreground'
const INACTIVE_CLASS = 'text-muted-foreground'

describe('AdminSidebar — active route detection', () => {
  it('highlights Dashboard only on exact /admin match', () => {
    renderSidebar('/admin')
    expect(linkClass('Dashboard')).toContain(ACTIVE_CLASS)
  })

  it('does NOT highlight Dashboard on /admin/mcp', () => {
    renderSidebar('/admin/mcp')
    expect(linkClass('Dashboard')).toContain(INACTIVE_CLASS)
    expect(linkClass('Dashboard')).not.toContain(ACTIVE_CLASS)
  })

  it('highlights MCP Servers on /admin/mcp', () => {
    renderSidebar('/admin/mcp')
    expect(linkClass('MCP Servers')).toContain(ACTIVE_CLASS)
  })

  it('highlights MCP Servers on a nested path like /admin/mcp/acme/my-server', () => {
    renderSidebar('/admin/mcp/acme/my-server')
    expect(linkClass('MCP Servers')).toContain(ACTIVE_CLASS)
  })

  it('highlights MCP Servers on /admin/mcp/new', () => {
    renderSidebar('/admin/mcp/new')
    expect(linkClass('MCP Servers')).toContain(ACTIVE_CLASS)
  })

  it('highlights Agents on /admin/agents', () => {
    renderSidebar('/admin/agents')
    expect(linkClass('Agents')).toContain(ACTIVE_CLASS)
  })

  it('highlights Agents on a nested agent path', () => {
    renderSidebar('/admin/agents/acme/my-agent')
    expect(linkClass('Agents')).toContain(ACTIVE_CLASS)
  })

  it('highlights Publishers on /admin/publishers', () => {
    renderSidebar('/admin/publishers')
    expect(linkClass('Publishers')).toContain(ACTIVE_CLASS)
  })

  it('highlights API Keys on /admin/api-keys', () => {
    renderSidebar('/admin/api-keys')
    expect(linkClass('API Keys')).toContain(ACTIVE_CLASS)
  })

  it('highlights Activity on /admin/audit', () => {
    renderSidebar('/admin/audit')
    expect(linkClass('Activity')).toContain(ACTIVE_CLASS)
  })

  it('only highlights one item at a time', () => {
    renderSidebar('/admin/mcp')
    const activeLinks = screen.getAllByRole('link').filter(el => el.className.includes(ACTIVE_CLASS))
    expect(activeLinks).toHaveLength(1)
  })

  it('renders all nav items', () => {
    renderSidebar('/admin')
    expect(screen.getByRole('link', { name: /dashboard/i })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /publishers/i })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /mcp servers/i })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /agents/i })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /reports/i })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /activity/i })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /api keys/i })).toBeInTheDocument()
  })
})
