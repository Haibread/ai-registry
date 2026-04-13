/**
 * breadcrumbs.test.tsx
 *
 * Tests for the Breadcrumbs navigation component. Segments with an href
 * render as links; the last segment renders as plain text.
 */

import { describe, it, expect } from "vitest"
import { render, screen } from "@testing-library/react"
import { MemoryRouter } from "react-router-dom"
import { Breadcrumbs } from "./breadcrumbs"

function renderBreadcrumbs(segments: { label: string; href?: string }[]) {
  return render(
    <MemoryRouter>
      <Breadcrumbs segments={segments} />
    </MemoryRouter>,
  )
}

describe("Breadcrumbs", () => {
  it("renders all segment labels", () => {
    renderBreadcrumbs([
      { label: "Home", href: "/" },
      { label: "Servers", href: "/servers" },
      { label: "My Server" },
    ])

    expect(screen.getByText("Home")).toBeInTheDocument()
    expect(screen.getByText("Servers")).toBeInTheDocument()
    expect(screen.getByText("My Server")).toBeInTheDocument()
  })

  it("renders intermediate segments with href as links", () => {
    renderBreadcrumbs([
      { label: "Home", href: "/" },
      { label: "Servers", href: "/servers" },
      { label: "My Server" },
    ])

    const homeLink = screen.getByText("Home").closest("a")
    expect(homeLink).toBeInTheDocument()
    expect(homeLink).toHaveAttribute("href", "/")

    const serversLink = screen.getByText("Servers").closest("a")
    expect(serversLink).toBeInTheDocument()
    expect(serversLink).toHaveAttribute("href", "/servers")
  })

  it("renders the last segment as plain text (not a link)", () => {
    renderBreadcrumbs([
      { label: "Home", href: "/" },
      { label: "My Server" },
    ])

    const lastSegment = screen.getByText("My Server")
    expect(lastSegment.closest("a")).toBeNull()
  })

  it("applies font-medium class to the last segment", () => {
    renderBreadcrumbs([
      { label: "Home", href: "/" },
      { label: "Current Page" },
    ])

    const lastSegment = screen.getByText("Current Page")
    expect(lastSegment.className).toContain("font-medium")
  })

  it("renders a single segment as plain text with font-medium", () => {
    renderBreadcrumbs([{ label: "Dashboard" }])

    const segment = screen.getByText("Dashboard")
    expect(segment.closest("a")).toBeNull()
    expect(segment.className).toContain("font-medium")
  })

  it("does not render the last segment as a link even if href is provided", () => {
    renderBreadcrumbs([
      { label: "Home", href: "/" },
      { label: "Current", href: "/current" },
    ])

    // The last segment should not be a link, even with href
    const lastSegment = screen.getByText("Current")
    expect(lastSegment.closest("a")).toBeNull()
  })

  it("has a nav element with Breadcrumb aria-label", () => {
    renderBreadcrumbs([{ label: "Home" }])
    expect(screen.getByLabelText("Breadcrumb")).toBeInTheDocument()
  })
})
