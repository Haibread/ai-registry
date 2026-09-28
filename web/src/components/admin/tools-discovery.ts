/**
 * Reconciles the tools a server returned through "Fetch from server" with the
 * list being edited. The review dialog renders the rows; `applyDiscovery`
 * turns the ticked rows into the next list.
 */

import type { MCPTool } from "./tools-schema"

export type DiscoveryStatus = "new" | "changed" | "same" | "missing"

export interface DiscoveryRow {
  name: string
  status: DiscoveryStatus
  /** The tool as currently edited; absent for `new`. */
  current?: MCPTool
  /** The tool as the server returned it; absent for `missing`. */
  discovered?: MCPTool
}

/** Key-order-insensitive JSON, so `{a,b}` and `{b,a}` compare equal. */
function canonical(v: unknown): string {
  if (Array.isArray(v)) return `[${v.map(canonical).join(",")}]`
  if (v !== null && typeof v === "object") {
    const entries = Object.entries(v as Record<string, unknown>)
      .filter(([, val]) => val !== undefined)
      .sort(([a], [b]) => (a < b ? -1 : a > b ? 1 : 0))
    return `{${entries.map(([k, val]) => `${JSON.stringify(k)}:${canonical(val)}`).join(",")}}`
  }
  return JSON.stringify(v)
}

/** Fields a server reports; an empty description equals an absent one. */
function comparable(t: MCPTool) {
  return {
    description: t.description || undefined,
    input_schema: t.input_schema,
    annotations: t.annotations,
  }
}

/**
 * One row per tool name: the current list's order first, then the tools only
 * the server has, in the server's order. Tools with an empty name are left out
 * of the reconciliation and survive untouched.
 */
export function diffTools(current: MCPTool[], discovered: MCPTool[]): DiscoveryRow[] {
  const byName = new Map(discovered.map((t) => [t.name, t]))
  const rows: DiscoveryRow[] = []
  const seen = new Set<string>()
  for (const cur of current) {
    if (!cur.name || seen.has(cur.name)) continue
    seen.add(cur.name)
    const disc = byName.get(cur.name)
    if (!disc) {
      rows.push({ name: cur.name, status: "missing", current: cur })
    } else {
      const same = canonical(comparable(cur)) === canonical(comparable(disc))
      rows.push({ name: cur.name, status: same ? "same" : "changed", current: cur, discovered: disc })
    }
  }
  for (const disc of discovered) {
    if (seen.has(disc.name)) continue
    seen.add(disc.name)
    rows.push({ name: disc.name, status: "new", discovered: disc })
  }
  return rows
}

/**
 * Rows ticked when the dialog opens: every new tool. A changed tool keeps the
 * edited version and a missing one stays, unless the user ticks them.
 */
export function defaultSelection(rows: DiscoveryRow[]): Set<string> {
  return new Set(rows.filter((r) => r.status === "new").map((r) => r.name))
}

/**
 * The list after applying the ticked rows: a ticked changed tool takes the
 * server's version, a ticked missing tool is removed, a ticked new tool is
 * appended. Everything else is kept as it is, fields the server does not know
 * about included.
 */
export function applyDiscovery(current: MCPTool[], rows: DiscoveryRow[], selected: Set<string>): MCPTool[] {
  const byName = new Map(rows.map((r) => [r.name, r]))
  const next: MCPTool[] = []
  for (const cur of current) {
    const row = cur.name ? byName.get(cur.name) : undefined
    if (!row || !selected.has(row.name)) {
      next.push(cur)
    } else if (row.status === "changed" && row.discovered) {
      const extra: MCPTool = { ...cur }
      delete extra.description
      delete extra.input_schema
      delete extra.annotations
      next.push({ ...extra, ...row.discovered })
    } else if (row.status !== "missing") {
      next.push(cur)
    }
  }
  for (const row of rows) {
    if (row.status === "new" && row.discovered && selected.has(row.name)) next.push(row.discovered)
  }
  return next
}

/** How many ticked rows actually change the list. */
export function countChanges(rows: DiscoveryRow[], selected: Set<string>): number {
  return rows.filter((r) => r.status !== "same" && selected.has(r.name)).length
}
