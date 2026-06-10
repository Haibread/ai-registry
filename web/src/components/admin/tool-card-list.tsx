/**
 * tool-card-list.tsx
 *
 * The structured "Form" tab of {@link import('./tools-editor').ToolsEditor}: a
 * list of editable tool cards over an `MCPTool[]` array, plus add / remove /
 * paste-from-`tools/list` controls.
 *
 * Each card structures the human-meaningful fields (`name`, `description`,
 * `annotations`) and keeps `input_schema` as a small JSON sub-editor — it *is*
 * JSON Schema, so a form-builder for it would be the wrong abstraction. Unknown
 * fields ride along untouched via the spread in `update`, which is what makes a
 * Form ↔ JSON round-trip lossless.
 */

import { useState } from "react"
import { Cpu, Plus, Trash2, ChevronDown, ChevronRight, Clipboard } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Badge } from "@/components/ui/badge"
import { Card, CardContent, CardHeader } from "@/components/ui/card"
import { parseTools, unwrapToolsList, type MCPTool } from "./tools-schema"

/**
 * The MCP spec's well-known boolean tool hints, offered as toggle badges. Any
 * other annotation key (a `title`, a vendor extension) arrives via paste / the
 * JSON tab and is shown read-only so it is never silently dropped.
 */
const KNOWN_HINTS = ["readOnlyHint", "destructiveHint", "idempotentHint", "openWorldHint"] as const

/** Monospace styling for the per-tool input_schema JSON sub-editor. */
const jsonTextareaClass =
  "w-full rounded-md border border-input bg-transparent px-3 py-2 text-xs font-mono shadow-xs " +
  "placeholder:text-muted-foreground focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-ring resize-y"

const emptyTool = (): MCPTool => ({ name: "" })

interface ToolCardListProps {
  tools: MCPTool[]
  onChange: (next: MCPTool[]) => void
}

export function ToolCardList({ tools, onChange }: ToolCardListProps) {
  const [pasteError, setPasteError] = useState<string | null>(null)

  const update = (i: number, patch: Partial<MCPTool>) =>
    onChange(tools.map((t, idx) => (idx === i ? { ...t, ...patch } : t)))
  const remove = (i: number) => onChange(tools.filter((_, idx) => idx !== i))
  const add = () => onChange([...tools, emptyTool()])

  // Read the clipboard, accept a bare array or a `tools/list` envelope, validate
  // through the shared parser, and append. Re-using parseTools guarantees the
  // Form and JSON tabs can never disagree about what is valid.
  async function pasteFromClipboard() {
    setPasteError(null)
    let text: string
    try {
      text = await navigator.clipboard.readText()
    } catch {
      setPasteError("Clipboard is unavailable — switch to the JSON tab and paste there.")
      return
    }
    const result = parseTools(unwrapToolsList(text))
    if (!result.ok) {
      setPasteError(result.error.message)
      return
    }
    onChange([...tools, ...result.tools])
  }

  return (
    <div className="space-y-3">
      {tools.length === 0 && (
        <p className="text-xs text-muted-foreground">
          No tools yet. Add one, or paste a <code className="font-mono">tools/list</code> response.
        </p>
      )}

      {tools.map((tool, i) => (
        // Index key: tools have no stable id before publish and the list is
        // short and edited in place, so positional identity is correct here.
        <ToolCard
          key={i}
          tool={tool}
          index={i}
          onChange={(patch) => update(i, patch)}
          onRemove={() => remove(i)}
        />
      ))}

      <div className="flex flex-wrap items-center gap-2">
        <Button type="button" size="sm" variant="outline" onClick={add} className="gap-1.5">
          <Plus className="h-3.5 w-3.5" aria-hidden="true" /> Add tool
        </Button>
        <Button type="button" size="sm" variant="outline" onClick={pasteFromClipboard} className="gap-1.5">
          <Clipboard className="h-3.5 w-3.5" aria-hidden="true" /> Paste from{" "}
          <code className="font-mono">tools/list</code>
        </Button>
      </div>
      {pasteError && (
        <p role="alert" className="text-xs text-destructive">
          {pasteError}
        </p>
      )}
    </div>
  )
}

interface ToolCardProps {
  tool: MCPTool
  index: number
  onChange: (patch: Partial<MCPTool>) => void
  onRemove: () => void
}

function ToolCard({ tool, index, onChange, onRemove }: ToolCardProps) {
  const [schemaOpen, setSchemaOpen] = useState(false)
  // Local text buffer so a half-typed schema doesn't clobber the parsed value
  // on every keystroke; we only push a parsed object up when it's valid.
  const [schemaText, setSchemaText] = useState(() =>
    tool.input_schema ? JSON.stringify(tool.input_schema, null, 2) : "",
  )
  const [schemaError, setSchemaError] = useState<string | null>(null)

  const annotations = (tool.annotations ?? {}) as Record<string, unknown>
  const extraKeys = Object.keys(annotations).filter(
    (k) => !(KNOWN_HINTS as readonly string[]).includes(k),
  )

  function toggleHint(key: string) {
    const next = { ...annotations }
    if (next[key]) delete next[key]
    else next[key] = true
    onChange({ annotations: Object.keys(next).length > 0 ? next : undefined })
  }

  function commitSchema(text: string) {
    setSchemaText(text)
    const t = text.trim()
    if (t === "") {
      setSchemaError(null)
      onChange({ input_schema: undefined })
      return
    }
    let parsed: unknown
    try {
      parsed = JSON.parse(t)
    } catch (e) {
      setSchemaError(`Invalid JSON: ${(e as Error).message}`)
      return
    }
    if (typeof parsed !== "object" || parsed === null || Array.isArray(parsed)) {
      setSchemaError("Input schema must be a JSON object.")
      return
    }
    setSchemaError(null)
    onChange({ input_schema: parsed as Record<string, unknown> })
  }

  const schemaFieldId = `tool-${index}-input-schema`

  return (
    <Card className="bg-muted/30">
      <CardHeader className="px-3 pt-3 pb-2">
        <div className="flex items-center gap-2">
          <Cpu className="h-3.5 w-3.5 shrink-0 text-muted-foreground" aria-hidden="true" />
          <Input
            aria-label={`Tool ${index + 1} name`}
            value={tool.name}
            onChange={(e) => onChange({ name: e.target.value })}
            placeholder="read_file"
            className="h-8 font-mono text-sm"
          />
          <Button
            type="button"
            size="icon"
            variant="ghost"
            aria-label={`Remove tool ${index + 1}`}
            onClick={onRemove}
            className="h-8 w-8 shrink-0 text-muted-foreground"
          >
            <Trash2 className="h-3.5 w-3.5" aria-hidden="true" />
          </Button>
        </div>
      </CardHeader>

      <CardContent className="space-y-2.5 px-3 pb-3">
        <Input
          aria-label={`Tool ${index + 1} description`}
          value={tool.description ?? ""}
          onChange={(e) => onChange({ description: e.target.value || undefined })}
          placeholder="Human-readable description"
          className="h-8 text-sm"
        />

        <div className="flex flex-wrap items-center gap-1.5">
          <span className="mr-1 text-xs text-muted-foreground">Annotations</span>
          {KNOWN_HINTS.map((key) => {
            const on = Boolean(annotations[key])
            return (
              <button
                key={key}
                type="button"
                onClick={() => toggleHint(key)}
                aria-pressed={on}
                className="rounded-full focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-ring"
              >
                <Badge
                  variant={on ? "secondary" : "outline"}
                  className={`cursor-pointer px-1.5 py-0 text-[10px] ${on ? "" : "opacity-50"}`}
                >
                  {key.replace("Hint", "")}
                </Badge>
              </button>
            )
          })}
          {extraKeys.map((k) => (
            <Badge
              key={k}
              variant="secondary"
              className="px-1.5 py-0 text-[10px]"
              title="Preserved from import"
            >
              {k}: {String(annotations[k])}
            </Badge>
          ))}
        </div>

        <div className="border-t pt-2">
          <button
            type="button"
            onClick={() => setSchemaOpen((v) => !v)}
            className="flex w-full items-center justify-between text-xs text-muted-foreground"
            aria-expanded={schemaOpen}
            aria-controls={schemaFieldId}
          >
            <span className="inline-flex items-center gap-1.5">
              {schemaOpen ? (
                <ChevronDown className="h-3 w-3" aria-hidden="true" />
              ) : (
                <ChevronRight className="h-3 w-3" aria-hidden="true" />
              )}
              Input schema <span className="font-mono text-muted-foreground/70">(JSON Schema)</span>
            </span>
            {schemaError ? (
              <span className="text-destructive">invalid</span>
            ) : tool.input_schema ? (
              <span className="text-green-600 dark:text-green-400">set</span>
            ) : null}
          </button>
          {schemaOpen && (
            <div className="mt-1.5 space-y-1">
              <textarea
                id={schemaFieldId}
                aria-label={`Tool ${index + 1} input schema (JSON)`}
                rows={6}
                spellCheck={false}
                value={schemaText}
                onChange={(e) => commitSchema(e.target.value)}
                placeholder={'{\n  "type": "object",\n  "properties": {}\n}'}
                className={jsonTextareaClass}
              />
              {schemaError && (
                <p role="alert" className="text-xs text-destructive">
                  {schemaError}
                </p>
              )}
            </div>
          )}
        </div>
      </CardContent>
    </Card>
  )
}
