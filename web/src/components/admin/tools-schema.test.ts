/**
 * tools-schema.test.ts
 *
 * Unit tests for the shared tools validator/serializer that backs both tabs of
 * the dual-mode ToolsEditor. Covers the happy path, every structural error with
 * its JSON path, lossless round-tripping of unknown fields, and the
 * `tools/list` envelope unwrap.
 */

import { describe, it, expect } from "vitest"
import {
  parseTools,
  serializeTools,
  formatToolsError,
  unwrapToolsList,
  type MCPTool,
} from "./tools-schema"

describe("parseTools", () => {
  it("treats empty / whitespace input as a valid empty array", () => {
    for (const raw of ["", "   ", "\n\t"]) {
      const r = parseTools(raw)
      expect(r).toEqual({ ok: true, tools: [] })
    }
  })

  it("accepts a well-formed tools array", () => {
    const r = parseTools(
      JSON.stringify([
        { name: "read_file", description: "Read a file", input_schema: { type: "object" } },
        { name: "write_file", annotations: { destructiveHint: true } },
      ]),
    )
    expect(r.ok).toBe(true)
    if (r.ok) expect(r.tools).toHaveLength(2)
  })

  it("preserves unknown fields untouched (lossless round-trip)", () => {
    const r = parseTools(JSON.stringify([{ name: "x", title: "X", vendorExtra: 7 }]))
    expect(r.ok).toBe(true)
    if (r.ok) expect(r.tools[0]).toMatchObject({ name: "x", title: "X", vendorExtra: 7 })
  })

  it("rejects malformed JSON with an Invalid JSON message", () => {
    const r = parseTools("[{ name: ]")
    expect(r.ok).toBe(false)
    if (!r.ok) expect(r.error.message).toMatch(/Invalid JSON/)
  })

  it("rejects a non-array top level", () => {
    const r = parseTools('{ "name": "x" }')
    expect(r.ok).toBe(false)
    if (!r.ok) expect(r.error.message).toMatch(/must be a JSON array/)
  })

  it("rejects a non-object tool with a path", () => {
    const r = parseTools('["nope"]')
    expect(r.ok).toBe(false)
    if (!r.ok) expect(r.error.path).toBe("tools[0]")
  })

  it("requires a non-empty name with a path", () => {
    for (const bad of [[{}], [{ name: "" }], [{ name: "   " }]]) {
      const r = parseTools(JSON.stringify(bad))
      expect(r.ok).toBe(false)
      if (!r.ok) {
        expect(r.error.path).toBe("tools[0].name")
        expect(r.error.message).toMatch(/name is required/)
      }
    }
  })

  it("rejects duplicate names", () => {
    const r = parseTools(JSON.stringify([{ name: "dup" }, { name: "dup" }]))
    expect(r.ok).toBe(false)
    if (!r.ok) {
      expect(r.error.path).toBe("tools[1].name")
      expect(r.error.message).toMatch(/Duplicate/)
    }
  })

  it("rejects a non-object input_schema with a path", () => {
    const r = parseTools(JSON.stringify([{ name: "x", input_schema: [1, 2] }]))
    expect(r.ok).toBe(false)
    if (!r.ok) expect(r.error.path).toBe("tools[0].input_schema")
  })

  it("rejects a non-object annotations with a path", () => {
    const r = parseTools(JSON.stringify([{ name: "x", annotations: "no" }]))
    expect(r.ok).toBe(false)
    if (!r.ok) expect(r.error.path).toBe("tools[0].annotations")
  })

  it("rejects a non-string description with a path", () => {
    const r = parseTools(JSON.stringify([{ name: "x", description: 5 }]))
    expect(r.ok).toBe(false)
    if (!r.ok) expect(r.error.path).toBe("tools[0].description")
  })

  it("adopts the spec's inputSchema spelling into input_schema", () => {
    const r = parseTools(JSON.stringify([{ name: "x", inputSchema: { type: "object" } }]))
    expect(r.ok).toBe(true)
    if (r.ok) expect(r.tools).toEqual([{ name: "x", input_schema: { type: "object" } }])
  })

  it("prefers an explicit input_schema over the alias and drops the alias", () => {
    const r = parseTools(
      JSON.stringify([
        { name: "x", input_schema: { type: "object" }, inputSchema: { type: "string" } },
      ]),
    )
    expect(r.ok).toBe(true)
    if (r.ok) expect(r.tools).toEqual([{ name: "x", input_schema: { type: "object" } }])
  })

  it("rejects a non-object inputSchema alias with a path", () => {
    const r = parseTools(JSON.stringify([{ name: "x", inputSchema: "no" }]))
    expect(r.ok).toBe(false)
    if (!r.ok) expect(r.error.path).toBe("tools[0].inputSchema")
  })
})

describe("serializeTools", () => {
  it("returns an empty string for no tools", () => {
    expect(serializeTools([])).toBe("")
  })

  it("round-trips with parseTools", () => {
    const tools: MCPTool[] = [{ name: "a", description: "first" }, { name: "b" }]
    const r = parseTools(serializeTools(tools))
    expect(r.ok).toBe(true)
    if (r.ok) expect(r.tools).toEqual(tools)
  })

  it("pretty-prints with two-space indentation", () => {
    // Array element indented one level (2 sp), its keys one level deeper (4 sp).
    const out = serializeTools([{ name: "a" }])
    expect(out).toContain("\n  {")
    expect(out).toContain('\n    "name": "a"')
  })
})

describe("formatToolsError", () => {
  it("prefixes the path when present", () => {
    expect(formatToolsError({ message: "name is required.", path: "tools[1].name" })).toBe(
      "tools[1].name: name is required.",
    )
  })

  it("omits the prefix when there is no path", () => {
    expect(formatToolsError({ message: "Tools must be a JSON array." })).toBe(
      "Tools must be a JSON array.",
    )
  })
})

describe("unwrapToolsList", () => {
  it("unwraps a { tools: [...] } envelope", () => {
    const out = unwrapToolsList('{ "tools": [{ "name": "a" }] }')
    const r = parseTools(out)
    expect(r.ok).toBe(true)
    if (r.ok) expect(r.tools).toEqual([{ name: "a" }])
  })

  it("passes a bare array through unchanged", () => {
    const raw = '[{ "name": "a" }]'
    expect(unwrapToolsList(raw)).toBe(raw)
  })

  it("passes non-JSON through unchanged", () => {
    expect(unwrapToolsList("not json")).toBe("not json")
  })
})
