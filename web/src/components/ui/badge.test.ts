/**
 * badge.test.ts
 *
 * Tests for the two pure mapping functions exported from badge-variants.ts.
 *
 * These functions are called on every list and detail page to determine
 * the colour of status and visibility badges. A wrong mapping (e.g. a
 * "deprecated" server showing green) is a visible regression.
 */

// @vitest-environment node

import { describe, it, expect } from "vitest"
import { statusVariant, visibilityVariant } from "./badge-variants"

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

  it("maps 'deleted' to outline (no fill)", () => {
    expect(statusVariant("deleted")).toBe("outline")
  })

  it("returns a distinct variant for each status", () => {
    const variants = new Set([
      statusVariant("published"),
      statusVariant("deprecated"),
      statusVariant("draft"),
      statusVariant("deleted"),
    ])
    // All four should be different — no two statuses share a colour.
    expect(variants.size).toBe(4)
  })
})

describe("visibilityVariant", () => {
  // Private carries the heavier mark (restricted content an admin must
  // spot at a glance); public is the registry's normal end state.
  it("maps 'public' to outline (quiet — the normal end state)", () => {
    expect(visibilityVariant("public")).toBe("outline")
  })

  it("maps 'private' to default (solid — the restricted state)", () => {
    expect(visibilityVariant("private")).toBe("default")
  })

  it("returns distinct variants for public and private", () => {
    expect(visibilityVariant("public")).not.toBe(visibilityVariant("private"))
  })
})
