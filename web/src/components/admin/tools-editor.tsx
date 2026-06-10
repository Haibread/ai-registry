/**
 * tools-editor.tsx
 *
 * Dual-mode editor for a version's publisher-declared `tools[]` array. One piece
 * of state (`MCPTool[]`) is rendered two ways:
 *
 *   - "Form" — structured cards ({@link ToolCardList}), the clean hand-authoring
 *     path.
 *   - "JSON" — a raw textarea, the paste-from-`tools/list` path.
 *
 * Both views funnel through {@link parseTools} / {@link serializeTools} so they
 * can never disagree about validity. The component emits a hidden
 * `<input name="tools">` carrying the serialized JSON, so the surrounding
 * uncontrolled `FormData` forms (`new-version-form.tsx`,
 * `pages/admin/mcp/new.tsx`) keep reading `fd.get('tools')` exactly as before —
 * no lift to a form library required.
 *
 * Switching out of the JSON tab is guarded: invalid JSON blocks the switch and
 * surfaces the offending path (e.g. `tools[2].input_schema`) rather than
 * deferring the failure to a submit-time 422.
 */

import { useState } from "react"
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs"
import { Label } from "@/components/ui/label"
import {
  parseTools,
  serializeTools,
  formatToolsError,
  type MCPTool,
} from "./tools-schema"
import { ToolCardList } from "./tool-card-list"

type Mode = "form" | "json"

/** Monospace styling for the raw JSON tab. */
const jsonTextareaClass =
  "w-full rounded-md border border-input bg-transparent px-3 py-2 text-xs font-mono shadow-xs " +
  "placeholder:text-muted-foreground focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-ring resize-y"

interface ToolsEditorProps {
  /** Form field name for the hidden input the parent FormData reads. */
  name?: string
  /** Initial tools, e.g. when editing an existing draft. Defaults to empty. */
  initialTools?: MCPTool[]
}

export function ToolsEditor({ name = "tools", initialTools = [] }: ToolsEditorProps) {
  const [mode, setMode] = useState<Mode>("form")
  const [tools, setTools] = useState<MCPTool[]>(initialTools)
  // Live buffer for the JSON tab; only adopted into `tools` on a clean switch.
  const [jsonText, setJsonText] = useState("")
  const [jsonError, setJsonError] = useState<string | null>(null)

  function handleModeChange(next: string) {
    const target = next as Mode
    if (mode === target) return

    if (mode === "json" && target === "form") {
      // Leaving JSON: re-parse. A bad buffer blocks the switch so the user
      // can't carry malformed input into the structured view.
      const result = parseTools(jsonText)
      if (!result.ok) {
        setJsonError(formatToolsError(result.error))
        return
      }
      setTools(result.tools)
      setJsonError(null)
    }

    if (mode === "form" && target === "json") {
      // Entering JSON: serialize the current structured state into the buffer.
      const text = serializeTools(tools)
      setJsonText(text)
      setJsonError(null)
    }

    setMode(target)
  }

  function handleJsonChange(text: string) {
    setJsonText(text)
    const result = parseTools(text)
    setJsonError(result.ok ? null : formatToolsError(result.error))
  }

  // The value the parent form submits. While editing JSON we forward the raw
  // buffer verbatim (the backstop parse in the submit handler still guards it);
  // otherwise we serialize the structured state.
  const serialized = mode === "json" ? jsonText : serializeTools(tools)
  const count = tools.length

  return (
    <div className="space-y-2">
      {/* Bridge to the surrounding FormData form. */}
      <input type="hidden" name={name} value={serialized} readOnly />

      <Tabs value={mode} onValueChange={handleModeChange}>
        <div className="flex flex-wrap items-center justify-between gap-2">
          <Label className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
            Tools (optional)
            {count > 0 && (
              <span className="ml-1.5 font-normal normal-case tracking-normal text-muted-foreground">
                · {count} {count === 1 ? "tool" : "tools"}
              </span>
            )}
          </Label>
          <TabsList className="h-8">
            <TabsTrigger value="form" className="text-xs">
              Form
            </TabsTrigger>
            <TabsTrigger value="json" className="text-xs">
              JSON
            </TabsTrigger>
          </TabsList>
        </div>

        <TabsContent value="form">
          <ToolCardList tools={tools} onChange={setTools} />
        </TabsContent>

        <TabsContent value="json">
          <textarea
            aria-label="Tools (JSON)"
            rows={10}
            spellCheck={false}
            value={jsonText}
            onChange={(e) => handleJsonChange(e.target.value)}
            placeholder={
              '[\n  {\n    "name": "read_file",\n    "description": "Read a file from disk",\n    "input_schema": { "type": "object" }\n  }\n]'
            }
            className={jsonTextareaClass}
          />
          {jsonError ? (
            <p role="alert" className="mt-1 text-xs text-destructive">
              {jsonError}
            </p>
          ) : (
            <p className="mt-1 text-xs text-muted-foreground">
              JSON array of tools (e.g. from <code className="font-mono">tools/list</code>). Each
              item needs a unique <code className="font-mono">name</code>.
            </p>
          )}
        </TabsContent>
      </Tabs>
    </div>
  )
}
