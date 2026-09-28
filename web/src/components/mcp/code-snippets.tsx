import { useState } from 'react'
import { CodeBlock } from '@/components/ui/code-block'
import { connectionSources, type ConnectionPackage, type ConnectionRemote } from '@/lib/mcp-host-configs'
import {
  SNIPPET_LANGUAGES,
  exampleToolCall,
  generateCodeSnippet,
  toSnippetConnection,
  type SnippetLanguage,
} from '@/lib/mcp-code-snippets'
import type { components } from '@/lib/schema'

interface MCPCodeSnippetsProps {
  serverName: string
  packages: ConnectionPackage[]
  remotes?: ConnectionRemote[]
  tools?: components['schemas']['MCPTool'][]
}

export function MCPCodeSnippets({ serverName, packages, remotes = [], tools }: MCPCodeSnippetsProps) {
  const [lang, setLang] = useState<SnippetLanguage>('python')
  const [sourceIndex, setSourceIndex] = useState(0)

  const sources = connectionSources(serverName, packages, remotes)
  const source = sources[Math.min(sourceIndex, sources.length - 1)]
  if (!source) return null

  const snippet = generateCodeSnippet(lang, toSnippetConnection(source.params), exampleToolCall(tools))

  return (
    <div className="space-y-3">
      <h3 className="text-sm font-semibold">Use from Code</h3>
      <p className="text-xs text-muted-foreground">
        Connect to this server from your own application with an official MCP SDK.
      </p>

      {sources.length > 1 && (
        <select
          value={sourceIndex}
          onChange={(e) => setSourceIndex(Number(e.target.value))}
          className="h-8 max-w-full rounded-md border border-input bg-background px-2 text-sm"
          aria-label="Select code connection"
        >
          {sources.map((s, i) => (
            <option key={i} value={i}>
              {s.label}
            </option>
          ))}
        </select>
      )}

      <div className="flex flex-wrap items-center gap-1 rounded-lg border p-1 w-fit" role="group" aria-label="Language">
        {SNIPPET_LANGUAGES.map((l) => (
          <button
            key={l.value}
            type="button"
            aria-pressed={lang === l.value}
            onClick={() => setLang(l.value)}
            className={`rounded-md px-3 py-1 text-xs font-medium transition-colors ${
              lang === l.value
                ? 'bg-primary text-primary-foreground'
                : 'text-muted-foreground hover:text-foreground hover:bg-accent'
            }`}
          >
            {l.label}
          </button>
        ))}
      </div>

      <div className="space-y-1">
        <p className="text-xs text-muted-foreground">Install</p>
        <CodeBlock value={snippet.install} copyLabel="Copy install" />
      </div>

      {snippet.code === null ? (
        <p className="rounded-md border border-dashed p-3 text-xs text-muted-foreground">
          The Rust SDK has no client for the legacy SSE transport. Use a Streamable HTTP endpoint
          of this server, or another SDK.
        </p>
      ) : (
        <CodeBlock value={snippet.code} copyLabel="Copy code" />
      )}
    </div>
  )
}
