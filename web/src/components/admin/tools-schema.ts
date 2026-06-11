/**
 * Shared validation + (de)serialization for the publisher-declared MCP tools
 * array.
 *
 * Both editing surfaces of {@link import('./tools-editor').ToolsEditor} — the
 * structured Form tab and the raw JSON tab — funnel through these helpers so
 * they can never disagree about what counts as valid. `parseTools` is also the
 * single backstop the JSON tab runs on every keystroke and the Form tab runs
 * when importing a pasted `tools/list` response.
 *
 * This mirrors the server-side `domain.ValidateTools` contract closely enough
 * to catch structural mistakes in the browser; the backend remains the source
 * of truth and re-validates on write.
 */

import type { components } from "@/lib/schema"

export type MCPTool = components["schemas"]["MCPTool"]

/** A validation failure, with an optional JSON path to the offending node. */
export interface ToolsError {
  message: string
  /** e.g. `tools[2].input_schema` — points the editor at the exact node. */
  path?: string
}

export type ToolsResult =
  | { ok: true; tools: MCPTool[] }
  | { ok: false; error: ToolsError }

/** True for a non-null, non-array object — the shape `input_schema` requires. */
function isPlainObject(v: unknown): v is Record<string, unknown> {
  return typeof v === "object" && v !== null && !Array.isArray(v)
}

/**
 * Parse and validate the raw tools text.
 *
 * An empty / whitespace-only string is valid and yields an empty array (the
 * field is optional). The MCP spec's `tools/list` response spells the schema
 * `inputSchema`; that alias is adopted into our `input_schema` field so a
 * pasted live response keeps its schemas. Other unknown properties on a tool
 * are preserved untouched so a round-trip through the Form tab is lossless.
 */
export function parseTools(raw: string): ToolsResult {
  const trimmed = raw.trim()
  if (trimmed === "") return { ok: true, tools: [] }

  let parsed: unknown
  try {
    parsed = JSON.parse(trimmed)
  } catch (e) {
    return { ok: false, error: { message: `Invalid JSON: ${(e as Error).message}` } }
  }

  if (!Array.isArray(parsed)) {
    return { ok: false, error: { message: "Tools must be a JSON array." } }
  }

  const seen = new Set<string>()
  const tools: MCPTool[] = []
  for (let i = 0; i < parsed.length; i++) {
    const tool = parsed[i]
    const at = `tools[${i}]`

    if (!isPlainObject(tool)) {
      return { ok: false, error: { message: "Each tool must be an object.", path: at } }
    }
    if (typeof tool.name !== "string" || tool.name.trim() === "") {
      return { ok: false, error: { message: "name is required.", path: `${at}.name` } }
    }
    if (seen.has(tool.name)) {
      return { ok: false, error: { message: `Duplicate tool name "${tool.name}".`, path: `${at}.name` } }
    }
    seen.add(tool.name)

    if (tool.description !== undefined && typeof tool.description !== "string") {
      return { ok: false, error: { message: "description must be a string.", path: `${at}.description` } }
    }
    if (tool.input_schema !== undefined && !isPlainObject(tool.input_schema)) {
      return { ok: false, error: { message: "input_schema must be a JSON object.", path: `${at}.input_schema` } }
    }
    if (tool.inputSchema !== undefined && !isPlainObject(tool.inputSchema)) {
      return { ok: false, error: { message: "inputSchema must be a JSON object.", path: `${at}.inputSchema` } }
    }
    if (tool.annotations !== undefined && !isPlainObject(tool.annotations)) {
      return { ok: false, error: { message: "annotations must be a JSON object.", path: `${at}.annotations` } }
    }

    // Adopt the spec spelling when ours is absent; drop the alias either way
    // so the stored tool never carries two competing schema keys.
    const { inputSchema, ...rest } = tool
    if (rest.input_schema === undefined && inputSchema !== undefined) {
      rest.input_schema = inputSchema
    }
    tools.push(rest as MCPTool)
  }

  return { ok: true, tools }
}

/** Serialize tools for the JSON tab / the hidden form input. Empty → "". */
export function serializeTools(tools: MCPTool[]): string {
  return tools.length === 0 ? "" : JSON.stringify(tools, null, 2)
}

/** Render a {@link ToolsError} as a single human-readable line. */
export function formatToolsError(error: ToolsError): string {
  return error.path ? `${error.path}: ${error.message}` : error.message
}

/**
 * Accept either a bare `MCPTool[]` array or the `{ "tools": [...] }` envelope a
 * live `tools/list` JSON-RPC response carries, returning text `parseTools` can
 * consume. Non-JSON input is returned unchanged so `parseTools` reports the
 * underlying syntax error.
 */
export function unwrapToolsList(raw: string): string {
  try {
    const v: unknown = JSON.parse(raw)
    if (isPlainObject(v) && Array.isArray(v.tools)) {
      return JSON.stringify(v.tools)
    }
  } catch {
    /* not JSON — let parseTools surface the error */
  }
  return raw
}
