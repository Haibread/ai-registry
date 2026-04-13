/**
 * LifecycleStepper — visual Draft → Published → Deprecated flow with
 * clickable transitions for admin detail pages.
 */

import { cn } from '@/lib/utils'
import { FileText, CheckCircle, AlertTriangle, XCircle } from 'lucide-react'

const stages = [
  { key: 'draft',      label: 'Draft',      icon: FileText,      color: 'text-muted-foreground' },
  { key: 'published',  label: 'Published',  icon: CheckCircle,   color: 'text-green-600' },
  { key: 'deprecated', label: 'Deprecated', icon: AlertTriangle,  color: 'text-yellow-600' },
  { key: 'deleted',    label: 'Deleted',    icon: XCircle,       color: 'text-destructive' },
] as const

type Status = typeof stages[number]['key']

interface LifecycleStepperProps {
  currentStatus: string
  /** Callback when the user clicks a stage to transition to it. */
  onTransition?: (targetStatus: Status) => void
  /** Set of statuses that are valid transition targets from the current status. */
  allowedTransitions?: string[]
  className?: string
}

/**
 * Determines valid transitions from the given status.
 */
function defaultAllowedTransitions(status: string): string[] {
  switch (status) {
    case 'draft':
      return ['published']
    case 'published':
      return ['deprecated']
    case 'deprecated':
      return ['published']
    case 'deleted':
      return []
    default:
      return []
  }
}

export function LifecycleStepper({
  currentStatus,
  onTransition,
  allowedTransitions,
  className,
}: LifecycleStepperProps) {
  const currentIdx = stages.findIndex((s) => s.key === currentStatus)
  const validTargets = allowedTransitions ?? defaultAllowedTransitions(currentStatus)

  return (
    <div className={cn('flex items-center gap-1', className)}>
      {stages.map((stage, i) => {
        const isCurrent = stage.key === currentStatus
        const isPast = i < currentIdx
        const isTarget = validTargets.includes(stage.key)
        const Icon = stage.icon
        const clickable = isTarget && !!onTransition

        return (
          <div key={stage.key} className="flex items-center gap-1">
            {i > 0 && (
              <div
                className={cn(
                  'h-px w-4 sm:w-8',
                  isPast || isCurrent ? 'bg-primary' : 'bg-border',
                )}
              />
            )}
            <button
              type="button"
              disabled={!clickable}
              onClick={() => clickable && onTransition!(stage.key)}
              className={cn(
                'flex items-center gap-1.5 rounded-md px-2 py-1 text-xs font-medium transition-colors',
                isCurrent && 'bg-primary/10 ring-1 ring-primary',
                clickable && 'cursor-pointer hover:bg-accent',
                !clickable && !isCurrent && 'opacity-50',
              )}
              title={
                clickable
                  ? `Transition to ${stage.label}`
                  : isCurrent
                    ? `Current status: ${stage.label}`
                    : stage.label
              }
            >
              <Icon className={cn('h-3.5 w-3.5', isCurrent ? stage.color : isPast ? 'text-primary' : 'text-muted-foreground')} />
              <span className="hidden sm:inline">{stage.label}</span>
            </button>
          </div>
        )
      })}
    </div>
  )
}
