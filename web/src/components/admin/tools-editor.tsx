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
import { ChevronRight, Terminal } from "lucide-react"
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs"
import { Label } from "@/components/ui/label"
import { CopyButton } from "@/components/ui/copy-button"
import { cn } from "@/lib/utils"
import {
  parseTools,
  serializeTools,
  unwrapToolsList,
  formatToolsError,
  type MCPTool,
} from "./tools-schema"
import { ToolCardList } from "./tool-card-list"

type Mode = "form" | "json"

// "How do I get this list?" recipes shown in the JSON tab. One per tooling
// situation: the Inspector CLI when Node is available, plain curl for remote
// servers, and a raw stdio pipe for local servers — the latter two need no
// Node at all. The editor accepts each command's output as-is (`{"tools":
// […]}` envelope and the spec's `inputSchema` spelling included), so every
// recipe is paste-ready. The curl handshake follows the Streamable HTTP
// transport spec: per-message POST, dual Accept header, and the optional
// Mcp-Session-Id assigned by the initialize response.
const HOW_TO_SECTIONS = [
  {
    title: "MCP Inspector CLI (needs Node / npx)",
    command: `# local (stdio) server — pass its launch command
npx @modelcontextprotocol/inspector --cli \\
  npx -y @scope/your-server --method tools/list

# remote server — pass its endpoint URL
npx @modelcontextprotocol/inspector --cli \\
  https://mcp.example.com/mcp \\
  --transport http --method tools/list`,
  },
  {
    title: "Remote server with curl (no Node required)",
    command: `# 1. initialize — note the Mcp-Session-Id response header, if any
curl -isS https://mcp.example.com/mcp \\
  -H 'Content-Type: application/json' \\
  -H 'Accept: application/json, text/event-stream' \\
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"curl","version":"0"}}}'

# 2. acknowledge — keep the Mcp-Session-Id header only if step 1 returned one
curl -sS https://mcp.example.com/mcp \\
  -H 'Content-Type: application/json' \\
  -H 'Accept: application/json, text/event-stream' \\
  -H 'Mcp-Session-Id: <from step 1>' \\
  -d '{"jsonrpc":"2.0","method":"notifications/initialized"}'

# 3. list tools — if the reply is an SSE stream, the JSON is on the "data:" line
curl -sS https://mcp.example.com/mcp \\
  -H 'Content-Type: application/json' \\
  -H 'Accept: application/json, text/event-stream' \\
  -H 'Mcp-Session-Id: <from step 1>' \\
  -d '{"jsonrpc":"2.0","id":2,"method":"tools/list"}'`,
  },
  {
    title: "Local stdio server from any shell (no Node required)",
    command: `# pipe the handshake straight into the server command;
# the response line carrying "id":2 is the tools list
printf '%s\\n' \\
  '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"shell","version":"0"}}}' \\
  '{"jsonrpc":"2.0","method":"notifications/initialized"}' \\
  '{"jsonrpc":"2.0","id":2,"method":"tools/list"}' \\
  | your-server-command`,
  },
]

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
  // The "How do I get this list?" disclosure below the editor. A panel rather
  // than a tooltip: three multi-line commands need copy buttons and must work
  // on touch, neither of which hover can offer.
  const [howToOpen, setHowToOpen] = useState(false)

  function handleModeChange(next: string) {
    const target = next as Mode
    if (mode === target) return

    if (mode === "json" && target === "form") {
      // Leaving JSON: re-parse. A bad buffer blocks the switch so the user
      // can't carry malformed input into the structured view.
      const result = parseTools(unwrapToolsList(jsonText))
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
    // unwrap first: a pasted live tools/list response arrives as the
    // {"tools": […]} envelope, which is fine input, not an error.
    const result = parseTools(unwrapToolsList(text))
    setJsonError(result.ok ? null : formatToolsError(result.error))
  }

  // The value the parent form submits. While editing valid JSON we forward
  // the parsed, normalized form (envelope unwrapped, spec field aliases
  // adopted); an invalid buffer is forwarded verbatim so the submit handler's
  // backstop parse reports the real syntax error.
  const jsonResult = mode === "json" ? parseTools(unwrapToolsList(jsonText)) : null
  const serialized =
    jsonResult !== null
      ? jsonResult.ok
        ? serializeTools(jsonResult.tools)
        : jsonText
      : serializeTools(tools)
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

      {/* Outside the Tabs on purpose: the recipes feed both surfaces — the
          JSON textarea and the Form tab's "Paste from tools/list" button —
          and a full-width callout is discoverable where a tab-local text
          link was not. */}
      <div className="pt-1">
        <button
          type="button"
          onClick={() => setHowToOpen((o) => !o)}
          aria-expanded={howToOpen}
          className="flex w-full items-center gap-2 rounded-md border border-dashed px-3 py-2 text-xs text-muted-foreground transition-colors hover:border-muted-foreground/50 hover:text-foreground"
        >
          <Terminal className="h-3.5 w-3.5 text-primary" aria-hidden="true" />
          <span className="font-medium">How do I get this list from my server?</span>
          <ChevronRight
            className={cn("ml-auto h-3.5 w-3.5 transition-transform", howToOpen && "rotate-90")}
            aria-hidden="true"
          />
        </button>
        {howToOpen && (
          <div className="mt-2 space-y-3 rounded-md border bg-muted/40 p-3">
            {HOW_TO_SECTIONS.map((s) => (
              <div key={s.title} className="space-y-1">
                <div className="flex items-center justify-between gap-2">
                  <p className="text-xs font-medium">{s.title}</p>
                  <CopyButton value={s.command} label={`Copy commands: ${s.title}`} />
                </div>
                <pre className="overflow-x-auto rounded bg-muted p-2 font-mono text-[10px] leading-snug">
                  {s.command}
                </pre>
              </div>
            ))}
            <p className="text-xs text-muted-foreground">
              Paste the output as-is — into the JSON tab or the Form tab&apos;s{" "}
              <span className="font-medium">Paste from tools/list</span> — the{" "}
              <code className="font-mono">{'{"tools": […]}'}</code> envelope and the spec&apos;s{" "}
              <code className="font-mono">inputSchema</code> spelling are both accepted.
            </p>
          </div>
        )}
      </div>
    </div>
  )
}
