import { useEffect, useRef, useState } from 'react'
import { Flag } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { getPublicClient } from '@/lib/api-client'

export type ReportableResourceType = 'mcp_server' | 'agent'
export type ReportIssueType =
  | 'broken'
  | 'misleading'
  | 'spam'
  | 'security'
  | 'licensing'
  | 'outdated'
  | 'duplicate'
  | 'other'

const ISSUE_TYPES: { value: ReportIssueType; label: string }[] = [
  { value: 'broken', label: 'Broken / does not work' },
  { value: 'misleading', label: 'Misleading description' },
  { value: 'spam', label: 'Spam' },
  { value: 'security', label: 'Security concern' },
  { value: 'licensing', label: 'Licensing issue' },
  { value: 'outdated', label: 'Outdated' },
  { value: 'duplicate', label: 'Duplicate entry' },
  { value: 'other', label: 'Other' },
]

interface ReportDialogProps {
  resourceType: ReportableResourceType
  resourceId: string
  /** Display name shown in the dialog header, e.g. "acme/my-server". */
  resourceLabel?: string
}

/**
 * "Report an issue" button + native-dialog modal. Any user (authenticated or
 * not) may submit a report; submissions are triaged in the admin queue.
 */
export function ReportDialog({ resourceType, resourceId, resourceLabel }: ReportDialogProps) {
  const dialogRef = useRef<HTMLDialogElement>(null)
  const [issueType, setIssueType] = useState<ReportIssueType>('broken')
  const [description, setDescription] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [success, setSuccess] = useState(false)

  function open() {
    setError(null)
    setSuccess(false)
    setIssueType('broken')
    setDescription('')
    dialogRef.current?.showModal()
  }
  function close() {
    dialogRef.current?.close()
  }

  useEffect(() => {
    const d = dialogRef.current
    if (!d) return
    function onCancel(e: Event) {
      // Allow Escape to close; nothing else.
      void e
    }
    d.addEventListener('cancel', onCancel)
    return () => d.removeEventListener('cancel', onCancel)
  }, [])

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setError(null)
    if (description.trim().length < 5) {
      setError('Please describe the issue in at least 5 characters.')
      return
    }
    if (description.length > 4000) {
      setError('Description must be at most 4000 characters.')
      return
    }
    setSubmitting(true)
    try {
      const api = getPublicClient()
      const { error: apiError } = await api.POST('/api/v1/reports', {
        body: {
          resource_type: resourceType,
          resource_id: resourceId,
          issue_type: issueType,
          description: description.trim(),
        },
      })
      if (apiError) {
        setError((apiError as { detail?: string })?.detail || 'Failed to submit report.')
        return
      }
      setSuccess(true)
    } catch {
      setError('Network error. Please try again.')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <>
      <Button type="button" variant="ghost" size="sm" onClick={open} aria-label="Report an issue">
        <Flag className="h-4 w-4" />
        <span className="ml-2">Report an issue</span>
      </Button>

      <dialog
        ref={dialogRef}
        aria-labelledby="report-dialog-title"
        className="rounded-lg border border-border bg-background p-0 text-foreground shadow-xl backdrop:bg-black/50 backdrop:backdrop-blur-sm w-full max-w-lg"
      >
        <form onSubmit={handleSubmit} className="flex flex-col gap-4 p-6">
          <div>
            <h2 id="report-dialog-title" className="text-lg font-semibold">
              Report an issue
            </h2>
            {resourceLabel && (
              <p className="text-sm text-muted-foreground mt-1">
                Reporting <span className="font-mono">{resourceLabel}</span>
              </p>
            )}
          </div>

          {success ? (
            <div className="rounded border border-green-500/40 bg-green-500/10 px-3 py-2 text-sm">
              Thanks — your report has been submitted for review.
            </div>
          ) : (
            <>
              <div className="flex flex-col gap-2">
                <label htmlFor="report-issue-type" className="text-sm font-medium">
                  Issue type
                </label>
                <select
                  id="report-issue-type"
                  value={issueType}
                  onChange={(e) => setIssueType(e.target.value as ReportIssueType)}
                  className="rounded-md border border-input bg-background px-3 py-2 text-sm"
                  disabled={submitting}
                >
                  {ISSUE_TYPES.map((t) => (
                    <option key={t.value} value={t.value}>
                      {t.label}
                    </option>
                  ))}
                </select>
              </div>

              <div className="flex flex-col gap-2">
                <label htmlFor="report-description" className="text-sm font-medium">
                  Description
                </label>
                <textarea
                  id="report-description"
                  value={description}
                  onChange={(e) => setDescription(e.target.value)}
                  rows={5}
                  minLength={5}
                  maxLength={4000}
                  placeholder="Tell us what's wrong. Include URLs, error messages, or anything useful for triage."
                  className="rounded-md border border-input bg-background px-3 py-2 text-sm font-sans"
                  disabled={submitting}
                  required
                />
                <p className="text-xs text-muted-foreground">
                  {description.length} / 4000
                </p>
              </div>

              {error && (
                <div role="alert" className="rounded border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
                  {error}
                </div>
              )}
            </>
          )}

          <div className="flex justify-end gap-2 pt-2">
            <Button type="button" variant="outline" onClick={close} disabled={submitting}>
              {success ? 'Close' : 'Cancel'}
            </Button>
            {!success && (
              <Button type="submit" disabled={submitting}>
                {submitting ? 'Submitting…' : 'Submit report'}
              </Button>
            )}
          </div>
        </form>
      </dialog>
    </>
  )
}
