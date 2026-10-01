/**
 * Token input for the MCP protocol revisions a version supports, with a
 * "Detect from server" action that asks the API to probe the remote endpoint
 * and offers the accepted revisions before replacing the list.
 */

import { useState, type KeyboardEvent } from 'react'
import { useMutation } from '@tanstack/react-query'
import { AlertCircle, Check, Loader2, Lock, Plus, RefreshCw, X } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { useAuthClient } from '@/lib/api-client'
import type { components } from '@/lib/schema'
import { isHttpUrl } from '@/lib/utils'

type Problem = components['schemas']['Problem']

export interface ProtocolVersionsDiscoverySource {
  namespace: string
  /** The form's transport (runtime); `stdio` disables detection. */
  transport: string
  remoteUrl: string
}

interface ProtocolVersionsInputProps {
  /** Name of the hidden input carrying the list as a JSON array. */
  name?: string
  defaultValue?: string[]
  discovery?: ProtocolVersionsDiscoverySource
}

const REVISION = /^\d{4}-\d{2}-\d{2}$/

function revisionError(value: string, current: string[]): string | null {
  if (!REVISION.test(value) || Number.isNaN(Date.parse(value))) {
    return 'Use a YYYY-MM-DD protocol revision, e.g. 2025-06-18.'
  }
  if (current.includes(value)) return `${value} is already in the list.`
  return null
}

const newestFirst = (list: string[]) => [...list].sort().reverse()

function hostOf(url: string): string {
  try {
    return new URL(url).host
  } catch {
    return url
  }
}

export function ProtocolVersionsInput({
  name = 'protocol_versions',
  defaultValue = ['2025-03-26'],
  discovery,
}: ProtocolVersionsInputProps) {
  const client = useAuthClient()
  const [tokens, setTokens] = useState<string[]>(() => newestFirst(defaultValue))
  const [draft, setDraft] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [focused, setFocused] = useState(false)
  // Revisions the last detection found: offered for review, then as suggestions.
  const [detected, setDetected] = useState<string[] | null>(null)
  const [reviewing, setReviewing] = useState(false)

  const remote = !!discovery && discovery.transport !== 'stdio'
  const canDetect = remote && isHttpUrl(discovery.remoteUrl)

  const detection = useMutation<string[], Problem>({
    mutationFn: async () => {
      const { data, error } = await client.POST('/api/v1/mcp/tool-discoveries', {
        body: {
          namespace: discovery!.namespace,
          url: discovery!.remoteUrl,
          transport: discovery!.transport as 'http' | 'sse' | 'streamable_http',
        },
      })
      if (error) throw error as Problem
      return data.supported_protocol_versions
    },
    onSuccess: (supported) => {
      setDetected(supported)
      setReviewing(true)
    },
  })

  function add(raw: string): boolean {
    const value = raw.trim()
    const problem = revisionError(value, tokens)
    if (problem) {
      setError(problem)
      return false
    }
    setTokens(newestFirst([...tokens, value]))
    setDraft('')
    setError(null)
    return true
  }

  function remove(value: string) {
    if (tokens.length === 1) {
      setError('Keep at least one revision.')
      return
    }
    setTokens(tokens.filter((t) => t !== value))
    setError(null)
  }

  function handleKeyDown(e: KeyboardEvent<HTMLInputElement>) {
    if (e.key === 'Enter') {
      e.preventDefault()
      add(draft)
    } else if (e.key === 'Backspace' && draft === '' && tokens.length > 1) {
      setTokens(tokens.slice(0, -1))
    }
  }

  const query = draft.trim()
  const suggestions = (detected ?? []).filter((v) => !tokens.includes(v) && v.startsWith(query))
  const offerTyped = query !== '' && !suggestions.includes(query) && !tokens.includes(query)
  const showList = focused && !reviewing && (suggestions.length > 0 || offerTyped)

  let help = 'Type a YYYY-MM-DD revision and press Enter.'
  if (canDetect) help = 'Type a YYYY-MM-DD revision and press Enter, or detect them from the server.'
  else if (discovery && !remote) help = 'stdio servers cannot be probed: add the revisions by hand.'

  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between gap-3">
        <Label htmlFor={`${name}-input`}>Protocol versions</Label>
        {discovery && (
          <Button
            type="button"
            variant="outline"
            size="sm"
            disabled={!canDetect || detection.isPending}
            title={canDetect ? undefined : 'Detection needs a remote endpoint URL'}
            onClick={() => {
              setError(null)
              detection.mutate()
            }}
          >
            {detection.isPending ? (
              <Loader2 className="h-3.5 w-3.5 animate-spin" aria-hidden="true" />
            ) : (
              <RefreshCw className="h-3.5 w-3.5" aria-hidden="true" />
            )}
            {detection.isPending ? 'Probing revisions…' : 'Detect from server'}
          </Button>
        )}
      </div>

      <input type="hidden" name={name} value={JSON.stringify(tokens)} />

      <div className="relative">
        <div className="flex min-h-9 flex-wrap items-center gap-1.5 rounded-md border border-input px-1.5 py-1 shadow-xs focus-within:ring-2 focus-within:ring-ring">
          {tokens.map((t) => (
            <span
              key={t}
              className="flex h-7 items-center gap-0.5 rounded bg-muted pl-2.5 pr-0.5 font-mono text-[13px]"
            >
              {t}
              <button
                type="button"
                aria-label={`Remove ${t}`}
                onClick={() => remove(t)}
                className="flex h-6 w-6 items-center justify-center rounded text-muted-foreground hover:bg-background hover:text-foreground"
              >
                <X className="h-3 w-3" aria-hidden="true" />
              </button>
            </span>
          ))}
          <input
            id={`${name}-input`}
            value={draft}
            placeholder="Add a revision…"
            autoComplete="off"
            onChange={(e) => {
              setDraft(e.target.value)
              setError(null)
            }}
            onKeyDown={handleKeyDown}
            onFocus={() => setFocused(true)}
            onBlur={() => {
              setFocused(false)
              // A revision typed but never confirmed would otherwise be lost on submit.
              if (draft.trim() !== '') add(draft)
            }}
            className="h-7 min-w-36 flex-1 bg-transparent px-1 font-mono text-[13px] outline-none placeholder:font-sans placeholder:text-muted-foreground"
          />
        </div>

        {showList && (
          <ul
            aria-label="Suggested revisions"
            className="absolute inset-x-0 top-full z-10 mt-1 rounded-md border bg-background p-1 shadow-md"
          >
            {suggestions.map((v) => (
              <li key={v}>
                <button
                  type="button"
                  // mousedown, not click: the input's blur would otherwise close the list first.
                  onMouseDown={(e) => {
                    e.preventDefault()
                    add(v)
                  }}
                  className="flex h-9 w-full items-center justify-between rounded px-2.5 text-left hover:bg-muted"
                >
                  <span className="font-mono text-[13px]">{v}</span>
                  <span className="text-xs text-green-700 dark:text-green-400">supported by the server</span>
                </button>
              </li>
            ))}
            {offerTyped && (
              <li>
                <button
                  type="button"
                  onMouseDown={(e) => {
                    e.preventDefault()
                    add(query)
                  }}
                  className="flex h-9 w-full items-center gap-1.5 rounded bg-primary/10 px-2.5 text-left text-[13px] text-primary"
                >
                  <Plus className="h-3.5 w-3.5" aria-hidden="true" />
                  Add “{query}”
                </button>
              </li>
            )}
          </ul>
        )}
      </div>

      {reviewing && detected && discovery && (
        <div role="status" className="space-y-2.5 rounded-lg border bg-muted/40 p-3">
          {detected.length > 0 ? (
            <>
              <p className="text-[13px]">
                <span className="font-medium">{hostOf(discovery.remoteUrl)}</span>{' '}
                <span className="text-muted-foreground">
                  accepts {detected.length} {detected.length === 1 ? 'revision' : 'revisions'}
                </span>
              </p>
              <div className="flex flex-wrap gap-1.5">
                {detected.map((v) => (
                  <span
                    key={v}
                    className="flex h-6 items-center gap-1 rounded-full bg-green-100 pl-1.5 pr-2 font-mono text-xs text-green-800 dark:bg-green-950 dark:text-green-300"
                  >
                    <Check className="h-3 w-3" aria-hidden="true" />
                    {v}
                  </span>
                ))}
              </div>
              <div className="flex gap-2">
                <Button
                  type="button"
                  size="sm"
                  onClick={() => {
                    setTokens(newestFirst(detected))
                    setReviewing(false)
                    setError(null)
                  }}
                >
                  Use {detected.length === 1 ? 'this one' : `these ${detected.length}`}
                </Button>
                <Button type="button" variant="ghost" size="sm" onClick={() => setReviewing(false)}>
                  Keep my list
                </Button>
              </div>
            </>
          ) : (
            <div className="flex items-center justify-between gap-3">
              <p className="text-[13px] text-muted-foreground">
                {hostOf(discovery.remoteUrl)} answered but accepted none of the probed revisions.
              </p>
              <Button type="button" variant="ghost" size="sm" onClick={() => setReviewing(false)}>
                Dismiss
              </Button>
            </div>
          )}
        </div>
      )}

      {detection.isError && <DetectionError problem={detection.error} />}

      {error ? (
        <p role="alert" className="text-xs text-destructive">
          {error}
        </p>
      ) : (
        !reviewing && <p className="text-xs text-muted-foreground">{help}</p>
      )}
    </div>
  )
}

function DetectionError({ problem }: { problem: Problem }) {
  const unauthorized = problem.type?.split('/').pop() === 'upstream-unauthorized'
  const Icon = unauthorized ? Lock : AlertCircle
  return (
    <div
      role="alert"
      className="flex gap-2.5 rounded-lg border border-amber-200 bg-amber-50 p-3 text-[13px] text-amber-900 dark:border-amber-900 dark:bg-amber-950 dark:text-amber-200"
    >
      <Icon className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
      <div className="min-w-0 space-y-0.5 break-words">
        <p className="font-medium">
          {unauthorized ? 'This server requires authentication.' : (problem.title ?? 'Detection failed.')}
        </p>
        <p>
          {unauthorized ? 'Only public servers can be probed.' : problem.detail} Add the revisions by hand.
        </p>
      </div>
    </div>
  )
}
