import { useState } from 'react'
import { useLocation, useNavigate, useSearchParams } from 'react-router-dom'
import { ArrowLeft, Search } from 'lucide-react'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import { CodeBlock } from '@/components/ui/code-block'
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '@/components/ui/table'
import {
  jsonRpcToolCall,
  otherAnnotations,
  toolBehaviors,
  toolParameters,
  toolTitle,
  type BehaviorKey,
  type BehaviorState,
  type MCPTool,
} from '@/lib/mcp-tools'
import { cn } from '@/lib/utils'

interface ToolsExplorerProps {
  tools: MCPTool[]
}

interface BehaviorCopy {
  label: string
  dot: string
  badge: string
  text: Record<Exclude<BehaviorState, 'not-applicable'>, string>
}

// `unset` describes the default the MCP spec tells clients to assume.
const BEHAVIOR: Record<BehaviorKey, BehaviorCopy> = {
  readOnly: {
    label: 'Read-only',
    dot: 'bg-green-600 dark:bg-green-400',
    badge: 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200',
    text: {
      yes: 'Does not modify its environment.',
      no: 'May modify its environment.',
      unset: 'Not declared: clients assume it may modify its environment.',
    },
  },
  destructive: {
    label: 'Destructive',
    dot: 'bg-red-600 dark:bg-red-400',
    badge: 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200',
    text: {
      yes: 'May delete or overwrite data.',
      no: 'Only makes additive changes.',
      unset: 'Not declared: clients assume it may delete or overwrite data.',
    },
  },
  idempotent: {
    label: 'Idempotent',
    dot: 'bg-blue-600 dark:bg-blue-400',
    badge: 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200',
    text: {
      yes: 'Repeating a call with the same arguments has no further effect.',
      no: 'Repeating a call may have further effect.',
      unset: 'Not declared: clients assume repeated calls may have further effect.',
    },
  },
  openWorld: {
    label: 'External',
    dot: 'bg-amber-500 dark:bg-amber-400',
    badge: 'bg-amber-100 text-amber-800 dark:bg-amber-900 dark:text-amber-200',
    text: {
      yes: 'Interacts with systems outside the server.',
      no: 'Stays within a closed domain.',
      unset: 'Not declared: clients assume it reaches outside systems.',
    },
  },
}

function matches(tool: MCPTool, query: string): boolean {
  const q = query.trim().toLowerCase()
  if (!q) return true
  return [tool.name, toolTitle(tool), tool.description]
    .some((s) => s?.toLowerCase().includes(q))
}

function countYes(tools: MCPTool[], key: BehaviorKey): number {
  return tools.filter((t) => toolBehaviors(t).some((b) => b.key === key && b.state === 'yes')).length
}

export function ToolsExplorer({ tools }: ToolsExplorerProps) {
  const [query, setQuery] = useState('')
  const [searchParams] = useSearchParams()
  const location = useLocation()
  const navigate = useNavigate()

  const filtered = tools.filter((t) => matches(t, query))
  const picked = tools.find((t) => t.name === searchParams.get('tool'))
  const selected = picked ?? filtered[0] ?? null

  // The selection lives in `?tool=` so a tool can be linked to; the hash
  // still carries the detail page's tab and must survive the navigation.
  const select = (name: string | null) => {
    const params = new URLSearchParams(searchParams)
    if (name) params.set('tool', name)
    else params.delete('tool')
    const search = params.toString()
    navigate({ search: search ? `?${search}` : '', hash: location.hash }, { replace: true })
  }

  const summary = [
    `${tools.length} ${tools.length === 1 ? 'tool' : 'tools'}`,
    `${countYes(tools, 'readOnly')} read-only`,
    `${countYes(tools, 'destructive')} destructive`,
  ].join(' · ')

  return (
    <div className="space-y-3">
      <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
        <div className="relative sm:w-72">
          <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" aria-hidden="true" />
          <Input
            type="search"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="Search tools…"
            aria-label="Search tools"
            className="h-9 pl-9"
          />
        </div>
        <p className="text-xs text-muted-foreground sm:ml-auto">{summary}</p>
      </div>

      <div className="grid gap-3 md:grid-cols-[18rem_minmax(0,1fr)]">
        {/* Below md the list and the detail are two screens: picking a tool
            swaps the list for its detail, "All tools" swaps back. */}
        <div className={cn('rounded-xl border md:block md:max-h-[36rem] md:overflow-y-auto', picked && 'hidden')}>
          {filtered.length === 0 ? (
            <p className="px-3 py-6 text-center text-sm text-muted-foreground">
              No tool matches “{query.trim()}”.
            </p>
          ) : (
            <ul aria-label="Tools" className="divide-y">
              {filtered.map((tool) => {
                const active = tool.name === selected?.name
                const flags = toolBehaviors(tool).filter((b) => b.state === 'yes')
                return (
                  <li key={tool.name}>
                    <button
                      type="button"
                      onClick={() => select(tool.name)}
                      aria-current={active ? 'true' : undefined}
                      className={cn(
                        'w-full cursor-pointer px-3 py-2.5 text-left transition-colors hover:bg-muted/60',
                        active && 'bg-muted shadow-[inset_3px_0_0_var(--color-primary)]',
                      )}
                    >
                      <span className="block truncate font-mono text-sm font-medium">{tool.name}</span>
                      {tool.description && (
                        <span className="block truncate text-xs text-muted-foreground">{tool.description}</span>
                      )}
                      {flags.length > 0 && (
                        <span className="mt-1.5 flex gap-1.5">
                          {flags.map((b) => (
                            <span
                              key={b.key}
                              title={BEHAVIOR[b.key].label}
                              className={cn('h-2 w-2 rounded-full', BEHAVIOR[b.key].dot)}
                            />
                          ))}
                          <span className="sr-only">{flags.map((b) => BEHAVIOR[b.key].label).join(', ')}</span>
                        </span>
                      )}
                    </button>
                  </li>
                )
              })}
            </ul>
          )}
        </div>

        {selected && (
          <div className={cn('min-w-0 md:block', !picked && 'hidden')}>
            <Button variant="ghost" size="sm" className="mb-2 -ml-2 md:hidden" onClick={() => select(null)}>
              <ArrowLeft className="h-4 w-4" aria-hidden="true" /> All tools
            </Button>
            <ToolDetail tool={selected} />
          </div>
        )}
      </div>
    </div>
  )
}

function ToolDetail({ tool }: { tool: MCPTool }) {
  const title = toolTitle(tool)
  const parameters = toolParameters(tool)
  const others = otherAnnotations(tool)

  return (
    <section aria-label={`Tool ${tool.name}`} className="space-y-4 rounded-xl border p-4 sm:p-5">
      <div className="space-y-1">
        <div className="flex flex-wrap items-baseline gap-x-2">
          <h3 className="break-all font-mono text-base font-semibold">{tool.name}</h3>
          {title && <span className="text-sm text-muted-foreground">{title}</span>}
        </div>
        {tool.description && <p className="text-sm text-muted-foreground">{tool.description}</p>}
      </div>

      <dl className="grid gap-2 sm:grid-cols-2">
        {toolBehaviors(tool).map(({ key, state }) => {
          const copy = BEHAVIOR[key]
          return (
            <div key={key} className={cn('rounded-lg border px-3 py-2', state !== 'yes' && 'text-muted-foreground')}>
              <dt>
                <span
                  className={cn(
                    'inline-flex rounded-md px-1.5 text-xs font-medium',
                    state === 'yes' ? copy.badge : 'bg-muted',
                  )}
                >
                  {state === 'yes' ? '✓' : '–'} {copy.label}
                </span>
              </dt>
              <dd className="mt-1 text-xs">
                {state === 'not-applicable' ? 'Not applicable to a read-only tool.' : copy.text[state]}
              </dd>
            </div>
          )
        })}
      </dl>

      {others.length > 0 && (
        <div className="flex flex-wrap items-center gap-1.5 text-xs">
          <span className="text-muted-foreground">Other annotations:</span>
          {others.map(([k, v]) => (
            <span key={k} className="rounded-md bg-muted px-1.5 font-mono">
              {k}: {JSON.stringify(v)}
            </span>
          ))}
        </div>
      )}

      <Tabs defaultValue="parameters">
        <TabsList>
          <TabsTrigger value="parameters">Parameters</TabsTrigger>
          <TabsTrigger value="example">Example call</TabsTrigger>
          <TabsTrigger value="schema">JSON schema</TabsTrigger>
        </TabsList>
        <TabsContent value="parameters" className="mt-3">
          {parameters === null ? (
            <p className="text-sm text-muted-foreground">
              This input schema composes other schemas and is shown as is in the JSON schema tab.
            </p>
          ) : parameters.length === 0 ? (
            <p className="text-sm text-muted-foreground">This tool takes no parameters.</p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Name</TableHead>
                  <TableHead>Type</TableHead>
                  <TableHead>Description</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {parameters.map((p) => (
                  <TableRow key={p.name}>
                    <TableCell className="align-top font-mono text-xs">
                      {p.name}
                      {p.required && (
                        <span className="ml-1.5 font-sans text-[10px] font-semibold uppercase text-red-700 dark:text-red-300">
                          required
                        </span>
                      )}
                    </TableCell>
                    <TableCell className="align-top font-mono text-xs text-blue-700 dark:text-blue-300">
                      {p.type ?? '—'}
                    </TableCell>
                    <TableCell className="align-top text-xs">
                      {p.description}
                      {p.enumValues && (
                        <div className="text-muted-foreground">
                          One of: <span className="font-mono">{p.enumValues.map((v) => JSON.stringify(v)).join(' · ')}</span>
                        </div>
                      )}
                      {p.defaultValue && (
                        <div className="text-muted-foreground">
                          Default: <span className="font-mono">{JSON.stringify(p.defaultValue.value)}</span>
                        </div>
                      )}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </TabsContent>
        <TabsContent value="example" className="mt-3 space-y-2">
          <p className="text-xs text-muted-foreground">
            The <span className="font-mono">tools/call</span> request an MCP client sends, with example values for the
            required arguments.
          </p>
          <CodeBlock value={jsonRpcToolCall(tool)} copyLabel="Copy example call" />
        </TabsContent>
        <TabsContent value="schema" className="mt-3">
          {tool.input_schema ? (
            <CodeBlock value={JSON.stringify(tool.input_schema, null, 2)} copyLabel="Copy JSON schema" />
          ) : (
            <p className="text-sm text-muted-foreground">This tool declares no input schema.</p>
          )}
        </TabsContent>
      </Tabs>
    </section>
  )
}
