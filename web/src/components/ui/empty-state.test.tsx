/**
 * empty-state.test.tsx
 *
 * Tests for the EmptyState placeholder component shown when a list
 * has no results or a section has no data.
 */

import { describe, it, expect } from "vitest"
import { render, screen } from "@testing-library/react"
import { EmptyState } from "./empty-state"

function TestIcon() {
  return <svg data-testid="test-icon" />
}

describe("EmptyState", () => {
  it("renders the title", () => {
    render(<EmptyState icon={<TestIcon />} title="No results found" />)
    expect(screen.getByText("No results found")).toBeInTheDocument()
  })

  it("renders the icon", () => {
    render(<EmptyState icon={<TestIcon />} title="No results found" />)
    expect(screen.getByTestId("test-icon")).toBeInTheDocument()
  })

  it("renders the description when provided", () => {
    render(
      <EmptyState
        icon={<TestIcon />}
        title="No results found"
        description="Try adjusting your search filters."
      />,
    )
    expect(
      screen.getByText("Try adjusting your search filters."),
    ).toBeInTheDocument()
  })

  it("does not render a description element when not provided", () => {
    const { container } = render(
      <EmptyState icon={<TestIcon />} title="No results found" />,
    )
    // Only two <p> elements: none for description
    const paragraphs = container.querySelectorAll("p")
    expect(paragraphs).toHaveLength(1) // just the title
  })

  it("renders an action when provided", () => {
    render(
      <EmptyState
        icon={<TestIcon />}
        title="No results found"
        action={<button>Create one</button>}
      />,
    )
    expect(screen.getByText("Create one")).toBeInTheDocument()
  })

  it("does not render an action wrapper when not provided", () => {
    const { container } = render(
      <EmptyState icon={<TestIcon />} title="No results found" />,
    )
    // The action wrapper has class "mt-1"; it should not be present
    expect(container.querySelector(".mt-1")).toBeNull()
  })
})
