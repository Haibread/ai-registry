/**
 * "Fetch from server" in the tools editor: the button's enablement, the review
 * dialog against a mocked API, and the list staying hand-editable afterwards.
 */

import { describe, it, expect, vi, beforeEach } from "vitest"
import { render, screen, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import type { ComponentProps } from "react"

const authFetch = vi.fn()
// Node's Request rejects the relative URLs the app's client uses in a browser.
vi.mock("@/lib/api-client", async () => {
  const { default: createClient } = await import("openapi-fetch")
  const client = createClient({ baseUrl: "http://registry.test", fetch: (req: Request) => authFetch(req) })
  return { useAuthClient: () => client }
})

import { ToolsEditor } from "./tools-editor"

beforeEach(() => {
  authFetch.mockReset()
  if (!HTMLDialogElement.prototype.showModal) {
    HTMLDialogElement.prototype.showModal = function () {
      this.setAttribute("open", "")
    }
  }
  if (!HTMLDialogElement.prototype.close) {
    HTMLDialogElement.prototype.close = function () {
      this.removeAttribute("open")
      this.dispatchEvent(new Event("close"))
    }
  }
})

function json(status: number, body: unknown, type = "application/json") {
  return new Response(JSON.stringify(body), { status, headers: { "Content-Type": type } })
}

const discovery = {
  endpoint: { url: "https://mcp.acme.dev/github", transport: "streamable_http" },
  attempts: [{ url: "https://mcp.acme.dev/github", transport: "streamable_http", status: 200 }],
  server_info: { name: "github-tools", version: "2.3.0" },
  protocol_version: "2025-06-18",
  tools: [
    { name: "list_issues", description: "List issues, filtered by state and labels." },
    { name: "search_code", description: "Search code across repositories.", annotations: { readOnlyHint: true } },
  ],
}

type Source = NonNullable<ComponentProps<typeof ToolsEditor>["discovery"]>

function renderEditor(source: Partial<Source> = {}, initialTools = [{ name: "list_issues", description: "List issues in a repository." }]) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const utils = render(
    <QueryClientProvider client={qc}>
      <ToolsEditor
        initialTools={initialTools}
        discovery={{
          namespace: "acme",
          transport: "streamable_http",
          remoteUrl: "https://mcp.acme.dev/github",
          ...source,
        }}
      />
    </QueryClientProvider>,
  )
  const hidden = () => JSON.parse((utils.container.querySelector('input[name="tools"]') as HTMLInputElement).value)
  return { ...utils, hidden }
}

describe("Fetch from server", () => {
  it.each([
    [{ transport: "stdio" }, /stdio server runs on the user's machine/],
    [{ remoteUrl: "" }, /Set a Remote URL/],
    [{ remoteUrl: "not a url" }, /Set a Remote URL/],
    [{ namespace: "" }, /Pick a publisher/],
  ])("is disabled for %o", (source, reason) => {
    renderEditor(source)
    expect(screen.getByRole("button", { name: /fetch from server/i })).toBeDisabled()
    expect(screen.getByText(reason)).toBeInTheDocument()
  })

  it("sends the form's values and adds only the new tools by default", async () => {
    authFetch.mockResolvedValue(json(200, discovery))
    const user = userEvent.setup()
    const { hidden } = renderEditor()

    await user.click(screen.getByRole("button", { name: /fetch from server/i }))
    const dialog = await screen.findByRole("dialog", { name: "Tools found on server" })
    await within(dialog).findByText("github-tools 2.3.0 · protocol 2025-06-18 · 2 tools")

    const req: Request = authFetch.mock.calls[0][0]
    expect(req.method).toBe("POST")
    expect(new URL(req.url).pathname).toBe("/api/v1/mcp/tool-discoveries")
    expect(await req.json()).toEqual({ namespace: "acme", url: "https://mcp.acme.dev/github", transport: "streamable_http" })

    expect(within(dialog).getByLabelText("Apply search_code")).toBeChecked()
    expect(within(dialog).getByLabelText("Apply list_issues")).not.toBeChecked()
    expect(within(dialog).getByText("Unticked: keeps your version.")).toBeInTheDocument()

    await user.click(within(dialog).getByRole("button", { name: "Apply 1 change" }))

    expect(hidden()).toEqual([
      { name: "list_issues", description: "List issues in a repository." },
      { name: "search_code", description: "Search code across repositories.", annotations: { readOnlyHint: true } },
    ])
    expect(screen.getByText("1 change applied from the server.")).toBeInTheDocument()

    // The applied list is an ordinary list: still editable by hand.
    const desc = screen.getByLabelText("Tool 2 description")
    await user.clear(desc)
    await user.type(desc, "Search code.")
    expect(hidden()[1].description).toBe("Search code.")
  })

  it("takes the server's version of a changed tool once ticked", async () => {
    authFetch.mockResolvedValue(json(200, discovery))
    const user = userEvent.setup()
    const { hidden } = renderEditor()

    await user.click(screen.getByRole("button", { name: /fetch from server/i }))
    const dialog = await screen.findByRole("dialog", { name: "Tools found on server" })
    await user.click(await within(dialog).findByLabelText("Apply list_issues"))
    await user.click(within(dialog).getByRole("button", { name: "Apply 2 changes" }))

    expect(hidden()[0].description).toBe("List issues, filtered by state and labels.")
  })

  it("explains a protected server and leaves the list alone", async () => {
    authFetch.mockResolvedValue(
      json(502, { type: "https://registry/errors/upstream-unauthorized", title: "Bad Gateway", status: 502 }, "application/problem+json"),
    )
    const user = userEvent.setup()
    const { hidden } = renderEditor()

    await user.click(screen.getByRole("button", { name: /fetch from server/i }))
    const alert = await screen.findByRole("alert")
    expect(alert).toHaveTextContent("This server requires authentication.")
    expect(alert).toHaveTextContent("Only public servers can be fetched.")
    expect(hidden()).toEqual([{ name: "list_issues", description: "List issues in a repository." }])
  })

  it("shows why each attempt failed", async () => {
    authFetch.mockResolvedValue(
      json(
        502,
        {
          type: "https://registry/errors/no-mcp-server",
          title: "Bad Gateway",
          status: 502,
          detail: "No MCP server answered. https://mcp.acme.dev/github (streamable_http): tls: x509: certificate signed by unknown authority; https://mcp.acme.dev/github (sse): tls: x509: certificate signed by unknown authority.",
          attempts: [
            { url: "https://mcp.acme.dev/github", transport: "streamable_http", status: 0, error: "tls: x509: certificate signed by unknown authority" },
            { url: "https://mcp.acme.dev/github", transport: "sse", status: 404, error: "HTTP 404" },
          ],
        },
        "application/problem+json",
      ),
    )
    const user = userEvent.setup()
    renderEditor()

    await user.click(screen.getByRole("button", { name: /fetch from server/i }))
    const alert = await screen.findByRole("alert")
    expect(alert).toHaveTextContent("No MCP server answered.")
    const lines = within(alert).getAllByRole("listitem")
    expect(lines.map((l) => l.textContent)).toEqual([
      "https://mcp.acme.dev/github · streamable_http: tls: x509: certificate signed by unknown authority",
      "https://mcp.acme.dev/github · sse: HTTP 404",
    ])
  })

  it("says so when the server declares no tools", async () => {
    authFetch.mockResolvedValue(json(200, { ...discovery, tools: [] }))
    const user = userEvent.setup()
    renderEditor({}, [])

    await user.click(screen.getByRole("button", { name: /fetch from server/i }))
    const dialog = await screen.findByRole("dialog", { name: "Tools found on server" })
    expect(await within(dialog).findByText("The server declares no tools.")).toBeInTheDocument()
    expect(within(dialog).getByRole("button", { name: "Apply 0 changes" })).toBeDisabled()
  })
})
