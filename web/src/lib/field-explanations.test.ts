/**
 * field-explanations.test.ts
 *
 * Tests for the field explanation lookup used by TooltipInfo components
 * across detail and listing pages.
 */

// @vitest-environment node

import { describe, it, expect } from "vitest"
import { getFieldExplanation, fieldExplanations } from "./field-explanations"

describe("getFieldExplanation", () => {
  it("returns the explanation for a known transport key", () => {
    expect(getFieldExplanation("stdio")).toBe(fieldExplanations.stdio)
  })

  it("returns the explanation for a known ecosystem key", () => {
    expect(getFieldExplanation("npm")).toBe(fieldExplanations.npm)
  })

  it("returns the explanation for a known visibility key", () => {
    expect(getFieldExplanation("public")).toBe(fieldExplanations.public)
  })

  it("returns the explanation for a MIME type key", () => {
    expect(getFieldExplanation("text/plain")).toBe(fieldExplanations["text/plain"])
  })

  it("returns the explanation for an auth scheme key", () => {
    expect(getFieldExplanation("Bearer")).toBe(fieldExplanations.Bearer)
  })

  it("returns the explanation for protocol_version", () => {
    expect(getFieldExplanation("protocol_version")).toBe(
      fieldExplanations.protocol_version,
    )
  })

  it("falls back to lowercase lookup", () => {
    // "NPM" is not a key, but "npm" is — the function tries lowercase
    expect(getFieldExplanation("NPM")).toBe(fieldExplanations.npm)
  })

  it("returns undefined for an unknown key", () => {
    expect(getFieldExplanation("totally_unknown_field")).toBeUndefined()
  })

  it("returns undefined for an empty string", () => {
    expect(getFieldExplanation("")).toBeUndefined()
  })
})
