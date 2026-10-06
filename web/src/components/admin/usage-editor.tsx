import { useRef, useState } from 'react'
import { useMutation } from '@tanstack/react-query'
import { toast } from 'sonner'
import { Bold, Braces, Code, Heading2, Info, Italic, Link2, List, SquareCode } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { ConfirmDialog } from '@/components/ui/confirm-dialog'
import { DirtyFormGuard } from '@/components/ui/dirty-form-guard'
import { MarkdownRenderer } from '@/components/ui/markdown-renderer'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { useAuthClient } from '@/lib/api-client'
import { cn, problemMessage } from '@/lib/utils'
import {
  MAX_USAGE_MARKDOWN_LENGTH,
  USAGE_VARIABLES,
  applyMarkdownFormat,
  insertText,
  renderUsageMarkdown,
  starterUsageMarkdown,
  usageValues,
  type MarkdownFormat,
  type TextEdit,
} from '@/lib/usage-markdown'
import type { components } from '@/lib/schema'

type MCPServer = components['schemas']['MCPServer']
type EditorView = 'write' | 'preview' | 'split'

interface UsageEditorProps {
  server: MCPServer
  canEdit: boolean
  /** Server Admins apply the edit; Editors submit it for review. */
  isServerAdmin: boolean
  /** Another change awaits review, so an Editor cannot submit one. */
  changePending: boolean
  onSaved: () => void
}

const FORMATS: { format: MarkdownFormat; label: string; icon: React.ReactNode }[] = [
  { format: 'heading', label: 'Heading', icon: <Heading2 className="h-4 w-4" /> },
  { format: 'bold', label: 'Bold', icon: <Bold className="h-4 w-4" /> },
  { format: 'italic', label: 'Italic', icon: <Italic className="h-4 w-4" /> },
  { format: 'link', label: 'Link', icon: <Link2 className="h-4 w-4" /> },
  { format: 'list', label: 'Bulleted list', icon: <List className="h-4 w-4" /> },
  { format: 'code', label: 'Inline code', icon: <Code className="h-4 w-4" /> },
  { format: 'codeblock', label: 'Code block', icon: <SquareCode className="h-4 w-4" /> },
]

export function UsageEditor({ server, canEdit, isServerAdmin, changePending, onSaved }: UsageEditorProps) {
  const api = useAuthClient()
  const current = server.usage_markdown ?? ''
  const values = usageValues(server)
  const [draft, setDraft] = useState<string | null>(null)
  const [view, setView] = useState<EditorView>('write')
  const [confirmReset, setConfirmReset] = useState(false)
  const textareaRef = useRef<HTMLTextAreaElement>(null)

  const editing = draft !== null
  const dirty = editing && draft !== current
  const tooLong = editing && draft.length > MAX_USAGE_MARKDOWN_LENGTH
  const blocked = changePending && !isServerAdmin

  const save = useMutation({
    mutationFn: async (usage_markdown: string) => {
      const { error } = await api.PATCH('/api/v1/mcp/servers/{namespace}/{slug}', {
        params: { path: { namespace: server.namespace, slug: server.slug } },
        body: { usage_markdown },
      })
      if (error) throw new Error(problemMessage(error, 'Failed to save usage instructions.'))
    },
    onSuccess: (_, usage) => {
      setDraft(null)
      setConfirmReset(false)
      onSaved()
      if (!isServerAdmin) toast.success('Submitted for review')
      else toast.success(usage ? 'Usage instructions saved' : 'Usage instructions reset')
    },
    onError: (e: Error) => toast.error(e.message),
  })

  const applyEdit = (edit: TextEdit) => {
    setDraft(edit.value)
    // Restore the selection once React has re-rendered the new value.
    requestAnimationFrame(() => {
      const el = textareaRef.current
      if (!el) return
      el.focus()
      el.setSelectionRange(edit.selectionStart, edit.selectionEnd)
    })
  }
  const withSelection = (fn: (value: string, start: number, end: number) => TextEdit) => {
    const el = textareaRef.current
    const value = draft ?? ''
    applyEdit(fn(value, el?.selectionStart ?? value.length, el?.selectionEnd ?? value.length))
  }

  const header = (
    <div className="flex flex-wrap items-center gap-2">
      <h2 id="usage-heading" className="text-lg font-semibold flex-1">Usage</h2>
      {dirty ? (
        <Badge variant="outline" className="border-amber-500/40 text-amber-700 dark:text-amber-400">Unsaved changes</Badge>
      ) : (
        <Badge variant="secondary">{current ? 'Custom' : 'Generated'}</Badge>
      )}
    </div>
  )

  if (!editing) {
    return (
      <section className="space-y-3" aria-labelledby="usage-heading">
        {header}
        {current ? (
          <div className="rounded-lg border p-4">
            <MarkdownRenderer content={renderUsageMarkdown(current, values)} />
          </div>
        ) : (
          <p className="flex gap-2 rounded-md border bg-muted/50 p-3 text-sm text-muted-foreground">
            <Info className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
            The public Usage tab shows instructions generated from the packages and remote endpoints of
            the latest version. Write your own Markdown to replace them — for example to document required
            environment variables or authentication.
          </p>
        )}
        {canEdit && (
          <div className="flex flex-wrap gap-2">
            <Button
              size="sm"
              variant={current ? 'outline' : 'default'}
              disabled={blocked}
              title={blocked ? 'A change is already pending review' : undefined}
              onClick={() => setDraft(current)}
            >
              {current ? 'Edit' : 'Customize'}
            </Button>
            {!current && (
              <Button size="sm" variant="outline" disabled={blocked} onClick={() => setDraft(starterUsageMarkdown(values))}>
                Start from generated
              </Button>
            )}
            {current && (
              <Button size="sm" variant="outline" disabled={blocked} onClick={() => setConfirmReset(true)}>
                Reset to generated
              </Button>
            )}
          </div>
        )}
        <ConfirmDialog
          open={confirmReset}
          onOpenChange={setConfirmReset}
          title="Reset usage instructions?"
          description={
            isServerAdmin
              ? 'Your custom Markdown will be deleted and the public Usage tab will show the generated instructions again.'
              : 'This submits the removal of your custom Markdown for review. Once approved, the public Usage tab shows the generated instructions again.'
          }
          confirmLabel={isServerAdmin ? 'Reset' : 'Submit for review'}
          destructive
          isPending={save.isPending}
          onConfirm={() => save.mutate('')}
        />
      </section>
    )
  }

  const preview = draft.trim() ? (
    <MarkdownRenderer content={renderUsageMarkdown(draft, values)} />
  ) : (
    <p className="text-sm text-muted-foreground">Nothing to preview.</p>
  )

  return (
    <section className="space-y-3" aria-labelledby="usage-heading">
      <DirtyFormGuard when={dirty} />
      {header}
      <p className="text-sm text-muted-foreground">
        Shown on the public Usage tab instead of the generated instructions, for every version of this server.
      </p>

      <div className="overflow-hidden rounded-lg border">
        <div className="flex flex-wrap items-center gap-0.5 border-b bg-muted/50 px-2 py-1">
          {FORMATS.map((f) => (
            <Button
              key={f.format}
              type="button"
              variant="ghost"
              size="icon"
              className="h-8 w-8"
              aria-label={f.label}
              title={f.label}
              disabled={view === 'preview'}
              onClick={() => withSelection((v, s, e) => applyMarkdownFormat(v, s, e, f.format))}
            >
              {f.icon}
            </Button>
          ))}
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button type="button" variant="ghost" size="sm" className="h-8 gap-1.5" disabled={view === 'preview'}>
                <Braces className="h-4 w-4" aria-hidden="true" />
                Insert variable
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="start">
              {USAGE_VARIABLES.map((v) => (
                <DropdownMenuItem
                  key={v.name}
                  className="flex-col items-start gap-0"
                  onSelect={() => withSelection((val, s, e) => insertText(val, s, e, `{{${v.name}}}`))}
                >
                  <span className="font-mono text-xs">{`{{${v.name}}}`}</span>
                  <span className="text-xs text-muted-foreground">
                    {v.description}
                    {values[v.name] ? ` — ${values[v.name]}` : ' — not available'}
                  </span>
                </DropdownMenuItem>
              ))}
            </DropdownMenuContent>
          </DropdownMenu>
          <div className="ml-auto flex rounded-md bg-background p-0.5" role="group" aria-label="Editor view">
            {(['write', 'preview', 'split'] as const).map((v) => (
              <button
                key={v}
                type="button"
                aria-pressed={view === v}
                onClick={() => setView(v)}
                className={cn(
                  'rounded px-2.5 py-1 text-xs font-medium capitalize text-muted-foreground transition-colors',
                  view === v && 'bg-muted text-foreground',
                  v === 'split' && 'hidden lg:inline-block',
                )}
              >
                {v}
              </button>
            ))}
          </div>
        </div>

        <div className={cn('grid', view === 'split' && 'lg:grid-cols-2')}>
          {view !== 'preview' && (
            <textarea
              ref={textareaRef}
              aria-label="Usage Markdown"
              value={draft}
              onChange={(e) => setDraft(e.target.value)}
              spellCheck
              className="min-h-72 w-full resize-y bg-background p-3 font-mono text-sm outline-none"
              placeholder={'## Getting started\n\nThis server needs a token…'}
            />
          )}
          {view !== 'write' && (
            <div className={cn('min-h-72 p-4', view === 'split' && 'border-t lg:border-t-0 lg:border-l')}>
              {preview}
            </div>
          )}
        </div>

        <div className="flex flex-wrap justify-between gap-2 border-t px-3 py-1.5 text-xs text-muted-foreground">
          <span>Markdown (GitHub flavored) · raw HTML is not rendered</span>
          <span className={cn(tooLong && 'font-medium text-destructive')}>
            {draft.length.toLocaleString()} / {MAX_USAGE_MARKDOWN_LENGTH.toLocaleString()} characters
          </span>
        </div>
      </div>

      <div className="flex flex-wrap gap-2">
        <Button size="sm" disabled={!dirty || tooLong || save.isPending} onClick={() => save.mutate(draft)}>
          {save.isPending ? 'Saving…' : isServerAdmin ? 'Save' : 'Submit for review'}
        </Button>
        <Button size="sm" variant="outline" onClick={() => setDraft(null)}>
          Cancel
        </Button>
      </div>
    </section>
  )
}
