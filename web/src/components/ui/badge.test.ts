/**
 * badge.test.ts
 *
 * Tests for the two pure mapping functions exported from badge.tsx.
 *
 * These functions are called on every list and detail page to determine
 * the colour of status and visibility badges. A wrong mapping (e.g. a
 * "deprecated" server showing green) is a visible regression.
 */

// @vitest-environment node

import { describe, it, expect } from "vitest"
import { statusVariant, visibilityVariant } from "./badge"

describe("statusVariant", () => {
  it("maps 'published' to success (green)", () => {
    expect(statusVariant("published")).toBe("success")
  })

  it("maps 'deprecated' to destructive (red)", () => {
    expect(statusVariant("deprecated")).toBe("destructive")
  })

  it("maps 'draft' to muted (grey)", () => {
    expect(statusVariant("draft")).toBe("muted")
  })

  it("returns a distinct variant for each status", () => {
    const variants = new Set([
      statusVariant("published"),
      statusVariant("deprecated"),
      statusVariant("draft"),
    ])
    // All three should be different — no two statuses share a colour.
    expect(variants.size).toBe(3)
  })
})

describe("visibilityVariant", () => {
  it("maps 'public' to default (primary colour)", () => {
    expect(visibilityVariant("public")).toBe("default")
  })

  it("maps 'private' to secondary (subdued colour)", () => {
    expect(visibilityVariant("private")).toBe("secondary")
  })

  it("returns distinct variants for public and private", () => {
    expect(visibilityVariant("public")).not.toBe(visibilityVariant("private"))
  })
})
