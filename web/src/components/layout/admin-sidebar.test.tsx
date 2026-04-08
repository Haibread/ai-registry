/**
 * admin-sidebar.test.tsx
 *
 * Tests for the active-route detection logic in AdminSidebar.
 *
 * The sidebar has two matching modes:
 *  - exact=true  (Dashboard): only highlights when pathname === "/admin"
 *  - exact=false (all others): highlights when pathname.startsWith(href)
 *
 * Getting this wrong means the wrong nav item is highlighted (or none is),
 * which is a confusing UX regression.
 */

import { render, screen } from "@testing-library/react"
import { describe, it, expect } from "vitest"
import { AdminSidebar } from "./admin-sidebar"

/** Returns the class string of the link with the given label. */
function linkClass(label: string): string {
  return screen.getByRole("link", { name: new RegExp(label, "i") }).className
}

// "shadow-sm" only appears on the active link; the inactive hover class
// "hover:bg-background/60" would false-positive on a plain "bg-background" check.
const ACTIVE_CLASS = "shadow-sm"
const INACTIVE_CLASS = "text-muted-foreground"

describe("AdminSidebar — active route detection", () => {
  it("highlights Dashboard only on exact /admin match", () => {
    render(<AdminSidebar pathname="/admin" />)
    expect(linkClass("Dashboard")).toContain(ACTIVE_CLASS)
  })

  it("does NOT highlight Dashboard on /admin/mcp", () => {
    render(<AdminSidebar pathname="/admin/mcp" />)
    expect(linkClass("Dashboard")).toContain(INACTIVE_CLASS)
    expect(linkClass("Dashboard")).not.toContain(ACTIVE_CLASS)
  })

  it("highlights MCP Servers on /admin/mcp", () => {
    render(<AdminSidebar pathname="/admin/mcp" />)
    expect(linkClass("MCP Servers")).toContain(ACTIVE_CLASS)
  })

  it("highlights MCP Servers on a nested path like /admin/mcp/acme/my-server", () => {
    render(<AdminSidebar pathname="/admin/mcp/acme/my-server" />)
    expect(linkClass("MCP Servers")).toContain(ACTIVE_CLASS)
  })

  it("highlights MCP Servers on /admin/mcp/new", () => {
    render(<AdminSidebar pathname="/admin/mcp/new" />)
    expect(linkClass("MCP Servers")).toContain(ACTIVE_CLASS)
  })

  it("highlights Agents on /admin/agents", () => {
    render(<AdminSidebar pathname="/admin/agents" />)
    expect(linkClass("Agents")).toContain(ACTIVE_CLASS)
  })

  it("highlights Agents on a nested agent path", () => {
    render(<AdminSidebar pathname="/admin/agents/acme/my-agent" />)
    expect(linkClass("Agents")).toContain(ACTIVE_CLASS)
  })

  it("highlights Publishers on /admin/publishers", () => {
    render(<AdminSidebar pathname="/admin/publishers" />)
    expect(linkClass("Publishers")).toContain(ACTIVE_CLASS)
  })

  it("highlights API Keys on /admin/api-keys", () => {
    render(<AdminSidebar pathname="/admin/api-keys" />)
    expect(linkClass("API Keys")).toContain(ACTIVE_CLASS)
  })

  it("only highlights one item at a time", () => {
    render(<AdminSidebar pathname="/admin/mcp" />)
    const activeLinks = screen
      .getAllByRole("link")
      .filter((el) => el.className.includes(ACTIVE_CLASS))
    expect(activeLinks).toHaveLength(1)
  })

  it("highlights nothing unexpected on an unknown sub-path", () => {
    render(<AdminSidebar pathname="/admin/mcp" />)
    // Dashboard should NOT be active (exact match only)
    expect(linkClass("Dashboard")).not.toContain(ACTIVE_CLASS)
    // Publishers, Agents, API Keys should NOT be active
    expect(linkClass("Publishers")).not.toContain(ACTIVE_CLASS)
    expect(linkClass("Agents")).not.toContain(ACTIVE_CLASS)
    expect(linkClass("API Keys")).not.toContain(ACTIVE_CLASS)
  })

  it("renders all five nav items regardless of pathname", () => {
    render(<AdminSidebar pathname="/admin" />)
    expect(screen.getByRole("link", { name: /dashboard/i })).toBeInTheDocument()
    expect(screen.getByRole("link", { name: /publishers/i })).toBeInTheDocument()
    expect(screen.getByRole("link", { name: /mcp servers/i })).toBeInTheDocument()
    expect(screen.getByRole("link", { name: /agents/i })).toBeInTheDocument()
    expect(screen.getByRole("link", { name: /api keys/i })).toBeInTheDocument()
  })
})
