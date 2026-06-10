/**
 * tools-editor.test.tsx
 *
 * Component tests for the dual-mode tools editor. The contract under test:
 *   - the hidden `tools` input always carries the serialized current state,
 *   - the Form and JSON tabs stay in sync through one source of truth,
 *   - leaving the JSON tab with invalid JSON is blocked (the switch is guarded),
 *   - annotation toggles and the per-tool input_schema sub-editor feed the
 *     serialized output.
 */

import { describe, it, expect } from "vitest"
import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { ToolsEditor } from "./tools-editor"

/** The hidden bridge input the surrounding FormData form reads. */
function hidden(container: HTMLElement): HTMLInputElement {
  return container.querySelector('input[name="tools"]') as HTMLInputElement
}

describe("ToolsEditor", () => {
  it("starts in Form mode with an empty hidden value", () => {
    const { container } = render(<ToolsEditor />)
    expect(screen.getByRole("tab", { name: "Form" })).toHaveAttribute("data-state", "active")
    expect(hidden(container).value).toBe("")
  })

  it("seeds the hidden value from initialTools", () => {
    const { container } = render(<ToolsEditor initialTools={[{ name: "read_file" }]} />)
    expect(JSON.parse(hidden(container).value)).toEqual([{ name: "read_file" }])
  })

  it("adds a tool via the Form tab and reflects it in the hidden input", async () => {
    const user = userEvent.setup()
    const { container } = render(<ToolsEditor />)

    await user.click(screen.getByRole("button", { name: /add tool/i }))
    await user.type(screen.getByLabelText("Tool 1 name"), "read_file")
    await user.type(screen.getByLabelText("Tool 1 description"), "Read a file")

    expect(JSON.parse(hidden(container).value)).toEqual([
      { name: "read_file", description: "Read a file" },
    ])
  })

  it("toggles a known annotation hint into the serialized output", async () => {
    const user = userEvent.setup()
    const { container } = render(<ToolsEditor initialTools={[{ name: "write_file" }]} />)

    await user.click(screen.getByRole("button", { name: "destructive" }))

    expect(JSON.parse(hidden(container).value)).toEqual([
      { name: "write_file", annotations: { destructiveHint: true } },
    ])
  })

  it("captures the per-tool input_schema sub-editor", async () => {
    const user = userEvent.setup()
    const { container } = render(<ToolsEditor initialTools={[{ name: "read_file" }]} />)

    await user.click(screen.getByRole("button", { name: /input schema/i }))
    const schema = screen.getByLabelText("Tool 1 input schema (JSON)")
    await user.clear(schema)
    // paste, not type: userEvent.type treats { and [ as special key syntax.
    await user.click(schema)
    await user.paste('{"type":"object"}')

    expect(JSON.parse(hidden(container).value)).toEqual([
      { name: "read_file", input_schema: { type: "object" } },
    ])
  })

  it("serializes form state into the JSON tab on switch", async () => {
    const user = userEvent.setup()
    render(<ToolsEditor initialTools={[{ name: "read_file", description: "Read" }]} />)

    await user.click(screen.getByRole("tab", { name: "JSON" }))

    const textarea = screen.getByLabelText("Tools (JSON)") as HTMLTextAreaElement
    expect(JSON.parse(textarea.value)).toEqual([{ name: "read_file", description: "Read" }])
  })

  it("hydrates the Form tab from edited JSON", async () => {
    const user = userEvent.setup()
    render(<ToolsEditor />)

    await user.click(screen.getByRole("tab", { name: "JSON" }))
    const textarea = screen.getByLabelText("Tools (JSON)")
    await user.clear(textarea)
    await user.click(textarea)
    await user.paste('[{"name":"forecast"}]')
    await user.click(screen.getByRole("tab", { name: "Form" }))

    expect(screen.getByRole("tab", { name: "Form" })).toHaveAttribute("data-state", "active")
    expect(screen.getByLabelText("Tool 1 name")).toHaveValue("forecast")
  })

  it("blocks leaving the JSON tab while the JSON is invalid", async () => {
    const user = userEvent.setup()
    const { container } = render(<ToolsEditor />)

    await user.click(screen.getByRole("tab", { name: "JSON" }))
    const textarea = screen.getByLabelText("Tools (JSON)")
    await user.click(textarea)
    await user.paste("{ not json")

    await user.click(screen.getByRole("tab", { name: "Form" }))

    // Still on JSON, with an error, and the hidden value is the raw buffer.
    expect(screen.getByRole("tab", { name: "JSON" })).toHaveAttribute("data-state", "active")
    expect(screen.getByRole("alert")).toBeInTheDocument()
    expect(hidden(container).value).toBe("{ not json")
  })

  it("reports the offending path for a structural error", async () => {
    const user = userEvent.setup()
    render(<ToolsEditor />)

    await user.click(screen.getByRole("tab", { name: "JSON" }))
    const textarea = screen.getByLabelText("Tools (JSON)")
    await user.clear(textarea)
    await user.click(textarea)
    await user.paste('[{"description":"no name"}]')

    expect(screen.getByRole("alert")).toHaveTextContent("tools[0].name")
  })
})
