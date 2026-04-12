/**
 * resource-icon.test.tsx
 *
 * Tests for the ResourceIcon component that renders a consistent icon
 * for each resource type (mcp-server, agent, publisher, skill, prompt).
 */

import { describe, it, expect } from "vitest"
import { render } from "@testing-library/react"
import { ResourceIcon } from "./resource-icon"
import type { ResourceType } from "./resource-icon"

const resourceTypes: ResourceType[] = [
  "mcp-server",
  "agent",
  "publisher",
  "skill",
  "prompt",
]

describe("ResourceIcon", () => {
  for (const type of resourceTypes) {
    it(`renders for resource type "${type}" without crashing`, () => {
      const { container } = render(<ResourceIcon type={type} />)
      const svg = container.querySelector("svg")
      expect(svg).toBeInTheDocument()
    })
  }

  it("applies aria-hidden to the icon", () => {
    const { container } = render(<ResourceIcon type="agent" />)
    const svg = container.querySelector("svg")
    expect(svg).toHaveAttribute("aria-hidden", "true")
  })

  it("applies custom className", () => {
    const { container } = render(
      <ResourceIcon type="mcp-server" className="h-8 w-8" />,
    )
    const svg = container.querySelector("svg")
    // Custom class should be applied (tailwind-merge may resolve h-4 vs h-8)
    expect(svg?.classList.toString()).toContain("h-8")
  })

  it("renders different icons for different resource types", () => {
    const { container: c1 } = render(<ResourceIcon type="mcp-server" />)
    const { container: c2 } = render(<ResourceIcon type="agent" />)

    const svg1 = c1.querySelector("svg")?.innerHTML
    const svg2 = c2.querySelector("svg")?.innerHTML

    // Different resource types should produce different SVG content
    expect(svg1).not.toBe(svg2)
  })
})
