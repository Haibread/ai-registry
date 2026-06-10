/**
 * DirtyFormGuard — unsaved-changes protection for forms (UI/UX review P2.5).
 *
 * Render it inside any form whose state would be lost on navigation and flip
 * `when` once the user has typed something. Two layers:
 *
 * - `beforeunload` covers refresh / tab close (browser-native prompt).
 * - A router blocker covers in-app navigation, confirming through the shared
 *   ConfirmDialog before the route changes.
 *
 * The router blocker needs a data router (the app root provides one). Unit
 * tests that render pages under a plain <MemoryRouter> degrade gracefully to
 * the beforeunload layer alone.
 */

import { useContext, useEffect } from 'react'
import { UNSAFE_DataRouterContext, useBlocker } from 'react-router-dom'
import { ConfirmDialog } from '@/components/ui/confirm-dialog'

interface DirtyFormGuardProps {
  /** Block navigation while true (the form has unsaved changes). */
  when: boolean
}

function useBeforeUnloadGuard(when: boolean) {
  useEffect(() => {
    if (!when) return
    const handler = (e: BeforeUnloadEvent) => {
      // Triggers the browser's generic "leave site?" prompt; custom text is
      // not supported by modern browsers.
      e.preventDefault()
    }
    window.addEventListener('beforeunload', handler)
    return () => window.removeEventListener('beforeunload', handler)
  }, [when])
}

function NavBlocker({ when }: DirtyFormGuardProps) {
  const blocker = useBlocker(
    ({ currentLocation, nextLocation }) =>
      when && currentLocation.pathname !== nextLocation.pathname,
  )

  return (
    <ConfirmDialog
      open={blocker.state === 'blocked'}
      onOpenChange={(o) => {
        if (!o) blocker.reset?.()
      }}
      title="Discard unsaved changes?"
      description="You have unsaved changes on this page. They will be lost if you leave."
      confirmLabel="Discard changes"
      cancelLabel="Keep editing"
      destructive
      onConfirm={() => blocker.proceed?.()}
    />
  )
}

export function DirtyFormGuard({ when }: DirtyFormGuardProps) {
  useBeforeUnloadGuard(when)
  // useBlocker throws outside a data router, so only mount the blocker when
  // one is present. The context read is the supported detection hook.
  const hasDataRouter = useContext(UNSAFE_DataRouterContext) != null
  return hasDataRouter ? <NavBlocker when={when} /> : null
}
