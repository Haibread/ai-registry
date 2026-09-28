import { describe, it, expect } from "vitest"
import { applyDiscovery, countChanges, defaultSelection, diffTools } from "./tools-discovery"
import type { MCPTool } from "./tools-schema"

const current: MCPTool[] = [
  { name: "list_issues", description: "List issues in a repository." },
  { name: "create_issue", description: "Open a new issue.", input_schema: { type: "object", required: ["title"] } },
  { name: "get_pull_request", description: "Fetch one pull request." },
]

const discovered: MCPTool[] = [
  { name: "list_issues", description: "List issues, filtered by state and labels." },
  { name: "create_issue", description: "Open a new issue.", input_schema: { required: ["title"], type: "object" } },
  { name: "search_code", description: "Search code across repositories.", annotations: { readOnlyHint: true } },
]

describe("diffTools", () => {
  it("classifies each tool by name, current order first", () => {
    const rows = diffTools(current, discovered)
    expect(rows.map((r) => [r.name, r.status])).toEqual([
      ["list_issues", "changed"],
      ["create_issue", "same"],
      ["get_pull_request", "missing"],
      ["search_code", "new"],
    ])
  })

  it("treats an empty description as absent", () => {
    const rows = diffTools([{ name: "a", description: "" }], [{ name: "a" }])
    expect(rows[0].status).toBe("same")
  })

  it("marks everything new on an empty list", () => {
    const rows = diffTools([], discovered)
    expect(rows.every((r) => r.status === "new")).toBe(true)
    expect(defaultSelection(rows)).toEqual(new Set(discovered.map((t) => t.name)))
  })

  it("leaves unnamed tools out of the reconciliation", () => {
    expect(diffTools([{ name: "" }], [])).toEqual([])
  })
})

describe("applyDiscovery", () => {
  const rows = diffTools(current, discovered)

  it("adds only new tools by default and keeps hand edits", () => {
    const next = applyDiscovery(current, rows, defaultSelection(rows))
    expect(next.map((t) => t.name)).toEqual(["list_issues", "create_issue", "get_pull_request", "search_code"])
    expect(next[0].description).toBe("List issues in a repository.")
  })

  it("takes the server's version of a ticked changed tool and keeps unknown fields", () => {
    const withExtra: MCPTool[] = [{ ...current[0], input_schema: { type: "object" }, vendor: 1 } as MCPTool, ...current.slice(1)]
    const r = diffTools(withExtra, discovered)
    const next = applyDiscovery(withExtra, r, new Set(["list_issues"]))
    expect(next[0]).toEqual({ name: "list_issues", description: "List issues, filtered by state and labels.", vendor: 1 })
  })

  it("removes a ticked missing tool", () => {
    const next = applyDiscovery(current, rows, new Set(["get_pull_request"]))
    expect(next.map((t) => t.name)).toEqual(["list_issues", "create_issue"])
  })

  it("keeps unnamed tools in place", () => {
    const withBlank = [...current, { name: "" }]
    const next = applyDiscovery(withBlank, diffTools(withBlank, discovered), new Set(["search_code"]))
    expect(next.map((t) => t.name)).toEqual(["list_issues", "create_issue", "get_pull_request", "", "search_code"])
  })
})

describe("countChanges", () => {
  it("ignores unchanged tools", () => {
    const rows = diffTools(current, discovered)
    expect(countChanges(rows, new Set(["create_issue", "search_code", "get_pull_request"]))).toBe(2)
  })
})
