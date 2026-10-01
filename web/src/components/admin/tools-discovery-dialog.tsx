/**
 * The "Fetch from server" review dialog of the tools editor: asks the API to
 * run tools/list against the version's remote URL and transport, exactly as
 * declared, shows how the result
 * differs from the list being edited, and applies only the ticked rows.
 */

import { useEffect, useRef, useState } from "react"
import { useQuery } from "@tanstack/react-query"
import { AlertCircle, Loader2, Lock } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Checkbox } from "@/components/ui/checkbox"
import { useAuthClient } from "@/lib/api-client"
import type { components } from "@/lib/schema"
import { cn } from "@/lib/utils"
import type { MCPTool } from "./tools-schema"
import {
  applyDiscovery,
  countChanges,
  defaultSelection,
  diffTools,
  type DiscoveryRow,
  type DiscoveryStatus,
} from "./tools-discovery"

type Discovery = components["schemas"]["MCPToolDiscovery"]
type Problem = components["schemas"]["MCPToolDiscoveryProblem"]

export interface DiscoveryRequest {
  namespace: string
  url: string
  transport: "http" | "sse" | "streamable_http"
}

interface ToolsDiscoveryDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  request: DiscoveryRequest
  currentTools: MCPTool[]
  /** Receives the next list and how many tools it changed. */
  onApply: (next: MCPTool[], changes: number) => void
}

export function ToolsDiscoveryDialog({ open, onOpenChange, ...rest }: ToolsDiscoveryDialogProps) {
  const ref = useRef<HTMLDialogElement>(null)

  useEffect(() => {
    const el = ref.current
    if (!el) return
    if (open && !el.open) el.showModal()
    else if (!open && el.open) el.close()
  }, [open])

  return (
    <dialog
      ref={ref}
      aria-label="Tools found on server"
      onClose={() => onOpenChange(false)}
      onClick={(e) => {
        if (e.target === ref.current) onOpenChange(false)
      }}
      className="m-auto w-full max-w-2xl rounded-lg border bg-background p-0 text-foreground shadow-lg backdrop:bg-black/50"
    >
      {/* Mounted only while open: each opening runs a fresh discovery. */}
      {open && <DiscoveryBody onClose={() => onOpenChange(false)} {...rest} />}
    </dialog>
  )
}

interface BodyProps extends Omit<ToolsDiscoveryDialogProps, "open" | "onOpenChange"> {
  onClose: () => void
}

function DiscoveryBody({ request, currentTools, onApply, onClose }: BodyProps) {
  const client = useAuthClient()
  const query = useQuery<Discovery, Problem>({
    queryKey: ["tool-discovery", request],
    queryFn: async () => {
      const { data, error } = await client.POST("/api/v1/mcp/tool-discoveries", { body: request })
      if (error) throw error as Problem
      return data
    },
    retry: false,
    gcTime: 0,
    staleTime: 0,
  })

  return (
    <div className="flex max-h-[85vh] flex-col gap-4 p-5">
      <div className="space-y-1">
        <h2 className="text-lg font-semibold">
          {query.isSuccess ? "Tools found on server" : "Fetch tools from server"}
        </h2>
        <p className="text-xs text-muted-foreground">
          {query.isSuccess ? serverLine(query.data) : <span className="font-mono">{request.url}</span>}
        </p>
      </div>

      {query.isPending && (
        <div role="status" className="flex items-center gap-2 rounded-md border p-3 text-sm">
          <Loader2 className="h-4 w-4 animate-spin text-primary" aria-hidden="true" />
          Connecting to the MCP server at {hostOf(request.url)}…
        </div>
      )}

      {query.isError && (
        <>
          <DiscoveryError problem={query.error} />
          <div className="flex justify-end gap-2 border-t pt-3">
            <Button type="button" variant="outline" size="sm" onClick={onClose}>
              Close
            </Button>
            <Button type="button" size="sm" onClick={() => query.refetch()}>
              Try again
            </Button>
          </div>
        </>
      )}

      {query.isSuccess && (
        <DiscoveryReview
          discovery={query.data}
          currentTools={currentTools}
          onApply={onApply}
          onClose={onClose}
        />
      )}
    </div>
  )
}

const STATUS_LABEL: Record<DiscoveryStatus, string> = {
  new: "new",
  changed: "changed",
  same: "same",
  missing: "not on server",
}

const STATUS_VARIANT = {
  new: "success",
  changed: "warning",
  same: "muted",
  missing: "destructive",
} as const

interface ReviewProps {
  discovery: Discovery
  currentTools: MCPTool[]
  onApply: (next: MCPTool[], changes: number) => void
  onClose: () => void
}

function DiscoveryReview({ discovery, currentTools, onApply, onClose }: ReviewProps) {
  const [rows] = useState(() => diffTools(currentTools, discovery.tools))
  const [selected, setSelected] = useState(() => defaultSelection(rows))
  const changes = countChanges(rows, selected)

  function toggle(name: string) {
    setSelected((prev) => {
      const next = new Set(prev)
      if (next.has(name)) next.delete(name)
      else next.add(name)
      return next
    })
  }

  const counts = rows.reduce<Record<DiscoveryStatus, number>>(
    (acc, r) => ({ ...acc, [r.status]: acc[r.status] + 1 }),
    { new: 0, changed: 0, same: 0, missing: 0 },
  )

  return (
    <>
      <details className="text-xs">
        <summary className="cursor-pointer text-muted-foreground">
          {discovery.attempts.length} {discovery.attempts.length === 1 ? "attempt" : "attempts"}
        </summary>
        <ul className="mt-2 space-y-1 rounded-md bg-muted p-2 font-mono">
          {discovery.attempts.map((a, i) => (
            <li key={i} className="flex gap-2">
              <span className="min-w-0 flex-1 truncate">
                {a.url} · {a.transport}
              </span>
              <span className={cn("shrink-0 font-semibold", a.error ? "text-muted-foreground" : "text-green-700 dark:text-green-400")}>
                {a.error ?? "ok"}
              </span>
            </li>
          ))}
        </ul>
      </details>

      {rows.length === 0 ? (
        <p className="rounded-md border p-3 text-sm">
          <span className="font-medium">The server declares no tools.</span>{" "}
          <span className="text-muted-foreground">Your list is unchanged.</span>
        </p>
      ) : (
        <>
          <div className="flex flex-wrap gap-1.5" aria-label="Summary">
            {(Object.keys(counts) as DiscoveryStatus[])
              .filter((s) => counts[s] > 0)
              .map((s) => (
                <Badge key={s} variant={STATUS_VARIANT[s]}>
                  {counts[s]} {STATUS_LABEL[s]}
                </Badge>
              ))}
          </div>
          <div className="min-h-0 overflow-auto rounded-md border">
            <table className="w-full min-w-[28rem] text-xs">
              <thead className="bg-muted text-left text-[11px] uppercase tracking-wide text-muted-foreground">
                <tr>
                  <th className="w-8 p-2" />
                  <th className="p-2">Tool</th>
                  <th className="p-2">Status</th>
                  <th className="p-2">Description</th>
                </tr>
              </thead>
              <tbody>
                {rows.map((r) => (
                  <DiscoveryTableRow key={r.name} row={r} checked={selected.has(r.name)} onToggle={() => toggle(r.name)} />
                ))}
              </tbody>
            </table>
          </div>
        </>
      )}

      <div className="flex flex-wrap items-center justify-between gap-2 border-t pt-3">
        <p className="text-xs text-muted-foreground">Nothing changes until you apply.</p>
        <div className="flex gap-2">
          <Button type="button" variant="outline" size="sm" onClick={onClose}>
            Cancel
          </Button>
          <Button
            type="button"
            size="sm"
            disabled={changes === 0}
            onClick={() => {
              onApply(applyDiscovery(currentTools, rows, selected), changes)
              onClose()
            }}
          >
            Apply {changes} {changes === 1 ? "change" : "changes"}
          </Button>
        </div>
      </div>
    </>
  )
}

function DiscoveryTableRow({ row, checked, onToggle }: { row: DiscoveryRow; checked: boolean; onToggle: () => void }) {
  const hint =
    row.status === "changed" && !checked
      ? "Unticked: keeps your version."
      : row.status === "missing"
        ? "Tick to remove it from your list."
        : null
  return (
    <tr className={cn("border-t align-top", row.status === "missing" && "text-muted-foreground")}>
      <td className="p-2">
        {row.status !== "same" && (
          <Checkbox checked={checked} onChange={onToggle} aria-label={`Apply ${row.name}`} />
        )}
      </td>
      <td className="whitespace-nowrap p-2 font-mono font-semibold">{row.name}</td>
      <td className="p-2">
        <Badge variant={STATUS_VARIANT[row.status]}>{STATUS_LABEL[row.status]}</Badge>
      </td>
      <td className="p-2">
        {row.status === "changed" && row.current?.description !== row.discovered?.description && (
          <span className="block text-muted-foreground line-through">{row.current?.description}</span>
        )}
        <span>{(row.discovered ?? row.current)?.description}</span>
        {hint && <span className="block text-muted-foreground">{hint}</span>}
      </td>
    </tr>
  )
}

function DiscoveryError({ problem }: { problem: Problem }) {
  const slug = problem.type?.split("/").pop()
  const { title, Icon } =
    slug === "upstream-unauthorized"
      ? { title: "This server requires authentication.", Icon: Lock }
      : slug === "blocked-address"
        ? { title: "This address is not allowed.", Icon: AlertCircle }
        : slug === "no-mcp-server"
          ? { title: "No MCP server answered.", Icon: AlertCircle }
          : slug === "timeout"
            ? { title: "The server did not answer in time.", Icon: AlertCircle }
            : { title: problem.title ?? "Fetching tools failed.", Icon: AlertCircle }
  const failed = (problem.attempts ?? []).filter((a) => a.error)
  // The attempts already say what the detail spells out, one line each.
  const detail =
    slug === "upstream-unauthorized"
      ? "Only public servers can be fetched. Paste the tools/list output or edit the list by hand."
      : failed.length > 0
        ? null
        : problem.detail
  return (
    <div role="alert" className="flex gap-2 rounded-md border border-destructive/40 bg-destructive/10 p-3 text-sm">
      <Icon className="mt-0.5 h-4 w-4 shrink-0 text-destructive" aria-hidden="true" />
      <div className="min-w-0 break-words">
        <p className="font-medium">{title}</p>
        {detail && <p className="text-xs text-muted-foreground">{detail}</p>}
        {failed.length > 0 && (
          <ul aria-label="Attempts" className="mt-1 space-y-0.5 font-mono text-xs text-muted-foreground">
            {failed.map((a, i) => (
              <li key={i}>
                {a.url} · {a.transport}: <span className="text-foreground">{a.error}</span>
              </li>
            ))}
          </ul>
        )}
        <p className="mt-1 text-xs text-muted-foreground">You can still add the tools by hand.</p>
      </div>
    </div>
  )
}

function serverLine(d: Discovery): string {
  const parts = []
  if (d.server_info) parts.push([d.server_info.name, d.server_info.version].filter(Boolean).join(" "))
  if (d.protocol_version) parts.push(`protocol ${d.protocol_version}`)
  parts.push(`${d.tools.length} ${d.tools.length === 1 ? "tool" : "tools"}`)
  return parts.join(" · ")
}

function hostOf(url: string): string {
  try {
    return new URL(url).host
  } catch {
    return url
  }
}
