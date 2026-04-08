/**
 * utils.test.ts
 *
 * Unit tests for src/lib/utils.ts — pure utility functions with no
 * network or DOM dependencies.
 */

// @vitest-environment node

import { describe, it, expect } from "vitest"
import { cn, formatDate } from "./utils"

// ── cn ────────────────────────────────────────────────────────────────────────

describe("cn", () => {
  it("returns a single class unchanged", () => {
    expect(cn("text-red-500")).toBe("text-red-500")
  })

  it("merges multiple classes", () => {
    expect(cn("p-4", "m-2")).toBe("p-4 m-2")
  })

  it("deduplicates Tailwind conflicts (last wins)", () => {
    // tailwind-merge resolves p-4 vs p-2: last one wins
    expect(cn("p-4", "p-2")).toBe("p-2")
  })

  it("ignores falsy values", () => {
    expect(cn("text-sm", false, undefined, null, "font-bold")).toBe(
      "text-sm font-bold"
    )
  })

  it("supports conditional objects", () => {
    expect(cn({ "text-red-500": true, "text-blue-500": false })).toBe(
      "text-red-500"
    )
  })

  it("supports array inputs", () => {
    expect(cn(["flex", "items-center"])).toBe("flex items-center")
  })

  it("returns empty string for no inputs", () => {
    expect(cn()).toBe("")
  })

  it("returns empty string for all-falsy inputs", () => {
    expect(cn(false, undefined, null)).toBe("")
  })
})

// ── formatDate ────────────────────────────────────────────────────────────────

describe("formatDate", () => {
  it("formats an ISO date string to human-readable form", () => {
    // 2025-01-15 should render as "Jan 15, 2025"
    const result = formatDate("2025-01-15T00:00:00Z")
    expect(result).toMatch(/Jan/)
    expect(result).toMatch(/15/)
    expect(result).toMatch(/2025/)
  })

  it("handles date-only strings", () => {
    const result = formatDate("2024-06-01")
    expect(result).toMatch(/2024/)
    expect(result).toMatch(/Jun/)
  })

  it("includes month, day, and year", () => {
    // Use noon UTC to avoid date shifting across timezone boundaries.
    const result = formatDate("2023-12-15T12:00:00Z")
    // Dec 15, 2023
    expect(result).toMatch(/Dec/)
    expect(result).toMatch(/15/)
    expect(result).toMatch(/2023/)
  })

  it("produces a non-empty string for any valid date", () => {
    const dates = [
      "2020-02-29T12:00:00Z", // leap day (noon to avoid TZ shift)
      "2000-01-15T12:00:00Z",
      "1999-06-15T12:00:00Z",
      "2099-07-04T12:00:00.000Z",
    ]
    for (const d of dates) {
      expect(formatDate(d).length).toBeGreaterThan(0)
    }
  })
})
