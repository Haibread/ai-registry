/**
 * copy-button.test.tsx
 *
 * Tests for the CopyButton component that copies a value to the clipboard
 * and shows visual feedback.
 */

import { describe, it, expect, vi, beforeEach } from "vitest"
import { render, screen, fireEvent, waitFor } from "@testing-library/react"
import { CopyButton } from "./copy-button"

beforeEach(() => {
  Object.assign(navigator, {
    clipboard: {
      writeText: vi.fn().mockResolvedValue(undefined),
    },
  })
})

describe("CopyButton", () => {
  it("renders without crashing", () => {
    render(<CopyButton value="hello" />)
    expect(screen.getByRole("button")).toBeInTheDocument()
  })

  it("has the correct aria-label by default", () => {
    render(<CopyButton value="hello" />)
    expect(screen.getByLabelText("Copy to clipboard")).toBeInTheDocument()
  })

  it("uses a custom label when provided", () => {
    render(<CopyButton value="hello" label="Copy URL" />)
    expect(screen.getByLabelText("Copy URL")).toBeInTheDocument()
  })

  it("calls navigator.clipboard.writeText with the value on click", async () => {
    render(<CopyButton value="test-value" />)
    fireEvent.click(screen.getByRole("button"))

    await waitFor(() => {
      expect(navigator.clipboard.writeText).toHaveBeenCalledWith("test-value")
    })
  })

  it("shows Check icon after clicking", async () => {
    const { container } = render(<CopyButton value="test-value" />)

    // Before click: the Copy icon is rendered (has a specific class)
    const svgBefore = container.querySelector("svg")
    expect(svgBefore).toBeInTheDocument()

    fireEvent.click(screen.getByRole("button"))

    // After click: the Check icon should appear with green color
    await waitFor(() => {
      const svg = container.querySelector("svg")
      expect(svg).toBeInTheDocument()
      expect(svg?.classList.toString()).toContain("text-green-600")
    })
  })
})
