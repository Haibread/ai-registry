import { cn } from '@/lib/utils'
import type { components } from '@/lib/schema'

type ActivityEvent = components['schemas']['PublisherActivityEvent']

type Tone = 'neutral' | 'info' | 'success' | 'warning' | 'danger'

const DOT: Record<Tone, string> = {
  neutral: 'bg-muted-foreground',
  info: 'bg-blue-500',
  success: 'bg-green-600',
  warning: 'bg-amber-500',
  danger: 'bg-destructive',
}

// describeActivity maps an audit action key (e.g. "mcp_server_version.published")
// to a tone + human verb. Keyed on the verb (the segment after the last dot) so
// the MCP and agent variants share one mapping.
function describeActivity(action: string): { tone: Tone; verb: string } {
  const verb = action.split('.').pop() ?? action
  switch (verb) {
    case 'published':
      return { tone: 'success', verb: 'published' }
    case 'approved':
      return { tone: 'success', verb: 'approved' }
    case 'deprecated':
      return { tone: 'warning', verb: 'deprecated' }
    case 'deletion_requested':
      return { tone: 'warning', verb: 'requested deletion of' }
    case 'rejected':
      return { tone: 'danger', verb: 'rejected' }
    case 'deletion_approved':
    case 'deleted':
      return { tone: 'danger', verb: 'deleted' }
    case 'submitted':
      return { tone: 'info', verb: 'submitted' }
    case 'visibility_changed':
      return { tone: 'info', verb: 'changed visibility of' }
    case 'created':
      return { tone: 'neutral', verb: 'created' }
    case 'updated':
      return { tone: 'neutral', verb: 'updated' }
    case 'withdrawn':
      return { tone: 'neutral', verb: 'withdrew' }
    case 'deletion_rejected':
      return { tone: 'neutral', verb: 'kept (deletion declined)' }
    default:
      return { tone: 'neutral', verb: verb.replace(/_/g, ' ') }
  }
}

// timeAgo renders a compact relative time ("12 minutes ago"). Uses the platform
// Intl.RelativeTimeFormat so it localises for free.
function timeAgo(iso: string): string {
  const diffMs = Date.now() - new Date(iso).getTime()
  const rtf = new Intl.RelativeTimeFormat(undefined, { numeric: 'auto' })
  const mins = Math.round(diffMs / 60_000)
  if (Math.abs(mins) < 60) return rtf.format(-mins, 'minute')
  const hrs = Math.round(mins / 60)
  if (Math.abs(hrs) < 24) return rtf.format(-hrs, 'hour')
  return rtf.format(-Math.round(hrs / 24), 'day')
}

// ActivityTimeline renders a publisher's recent audit events as a vertical
// timeline: a hairline rail with a tone-coloured dot per event, the actor, a
// human verb, and the target. Items fade in with a small stagger; the stagger
// collapses to an instant render under prefers-reduced-motion.
export function ActivityTimeline({ events }: { events: ActivityEvent[] }) {
  return (
    <ol>
      {events.map((e, i) => {
        const { tone, verb } = describeActivity(e.action)
        const target = e.version ? `${e.resource_slug}@${e.version}` : e.resource_slug
        const last = i === events.length - 1
        return (
          <li
            key={e.id}
            className="grid grid-cols-[auto_1fr] gap-3 animate-in fade-in-0 slide-in-from-bottom-1 fill-mode-both motion-reduce:animate-none"
            style={{ animationDelay: `${Math.min(i, 8) * 40}ms` }}
          >
            <div className="flex flex-col items-center pt-1.5">
              <span className={cn('h-2.5 w-2.5 rounded-full', DOT[tone])} aria-hidden="true" />
              {!last && <span className="my-1 w-px flex-1 bg-border" />}
            </div>
            <div className={last ? '' : 'pb-4'}>
              <p className="text-sm">
                <span className="font-medium">{e.actor_email || 'Someone'}</span> {verb}{' '}
                {e.resource_slug && <span className="font-mono text-xs">{target}</span>}
              </p>
              <p className="mt-0.5 text-xs text-muted-foreground">{timeAgo(e.created_at)}</p>
            </div>
          </li>
        )
      })}
    </ol>
  )
}
