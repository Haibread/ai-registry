/**
 * FilterBar — live filtering without a submit button.
 *
 * Selects update the URL immediately on change.
 * Text inputs are debounced (300 ms) so the URL only updates after the user
 * stops typing, avoiding a request on every keystroke.
 *
 * URL params are updated with navigate(..., { replace: true }) so the browser
 * back button skips intermediate filter states (only the final value is
 * recorded in history).
 *
 * Usage:
 *   <FilterBar
 *     q={q}
 *     namespace={namespace}
 *     status={status}
 *     visibility={visibility}
 *     statusOptions={["published", "deprecated"]}
 *     showVisibility          // admin only
 *     searchPlaceholder="Search servers…"
 *   />
 */

import { useNavigate, useLocation } from 'react-router-dom'
import { useCallback, useEffect, useRef, useState } from 'react'
import { Search, X } from 'lucide-react'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'

interface FilterBarProps {
  q?: string
  namespace?: string
  status?: string
  visibility?: string
  statusOptions: string[]
  showVisibility?: boolean
  searchPlaceholder?: string
}

const selectClass =
  'h-9 rounded-md border border-input bg-background px-3 text-sm ' +
  'text-foreground shadow-xs transition-colors focus-visible:outline-hidden ' +
  'focus-visible:ring-1 focus-visible:ring-ring min-w-[130px]'

const DEBOUNCE_MS = 300

export function FilterBar({
  q: initialQ = '',
  namespace: initialNamespace = '',
  status: initialStatus = '',
  visibility: initialVisibility = '',
  statusOptions,
  showVisibility = false,
  searchPlaceholder = 'Search…',
}: FilterBarProps) {
  const navigate = useNavigate()
  const { pathname, search } = useLocation()
  const searchParams = new URLSearchParams(search)

  // Derive initial state from the URL (source of truth), falling back to props.
  const [q, setQ] = useState(() => searchParams.get('q') ?? initialQ)
  const [namespace, setNamespace] = useState(
    () => searchParams.get('namespace') ?? initialNamespace
  )

  // Keep local text state in sync when the URL changes externally (browser
  // back/forward).
  /* eslint-disable react-hooks/set-state-in-effect */
  useEffect(() => {
    const p = new URLSearchParams(search)
    setQ(p.get('q') ?? '')
    setNamespace(p.get('namespace') ?? '')
  }, [search])
  /* eslint-enable react-hooks/set-state-in-effect */

  // Debounce timer ref — shared across q and namespace.
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  /** Build a new URLSearchParams from the current URL, overriding changed keys.
   *  Keys with empty string values are removed so the URL stays clean. */
  const buildParams = useCallback(
    (overrides: Record<string, string>) => {
      const p = new URLSearchParams(search)
      // Always reset cursor when any filter changes.
      p.delete('cursor')
      for (const [key, value] of Object.entries(overrides)) {
        if (value) {
          p.set(key, value)
        } else {
          p.delete(key)
        }
      }
      return p
    },
    [search]
  )

  /** Navigate immediately — used for selects. */
  const applyNow = useCallback(
    (overrides: Record<string, string>) => {
      if (debounceRef.current) clearTimeout(debounceRef.current)
      navigate(`${pathname}?${buildParams(overrides).toString()}`, { replace: true })
    },
    [navigate, pathname, buildParams]
  )

  /** Navigate after DEBOUNCE_MS — used for text inputs. */
  const applyDebounced = useCallback(
    (overrides: Record<string, string>) => {
      if (debounceRef.current) clearTimeout(debounceRef.current)
      debounceRef.current = setTimeout(() => {
        navigate(`${pathname}?${buildParams(overrides).toString()}`, { replace: true })
      }, DEBOUNCE_MS)
    },
    [navigate, pathname, buildParams]
  )

  // Cleanup on unmount.
  useEffect(() => () => { if (debounceRef.current) clearTimeout(debounceRef.current) }, [])

  const currentStatus = searchParams.get('status') ?? ''
  const currentVisibility = searchParams.get('visibility') ?? ''
  const hasFilters = !!(q || namespace || currentStatus || currentVisibility)

  return (
    // The form still works as a GET form when JS is unavailable.
    <form
      className="flex flex-wrap gap-2 items-center"
      onSubmit={(e) => {
        e.preventDefault()
        applyNow({ q, namespace })
      }}
    >
      {/* Full-text search */}
      <div className="relative flex-1 min-w-[200px] max-w-xs">
        <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground pointer-events-none" aria-hidden="true" />
        <Input
          name="q"
          value={q}
          onChange={(e) => {
            setQ(e.target.value)
            applyDebounced({ q: e.target.value, namespace })
          }}
          placeholder={searchPlaceholder}
          className="pl-9"
          autoComplete="off"
          aria-label="Search"
        />
      </div>

      {/* Namespace / publisher */}
      <Input
        name="namespace"
        value={namespace}
        onChange={(e) => {
          setNamespace(e.target.value)
          applyDebounced({ q, namespace: e.target.value })
        }}
        placeholder="Publisher…"
        className="min-w-[140px] max-w-[180px]"
        aria-label="Filter by publisher"
        autoComplete="off"
      />

      {/* Status — instant */}
      <select
        name="status"
        value={currentStatus}
        onChange={(e) => applyNow({ status: e.target.value })}
        className={selectClass}
        aria-label="Filter by status"
      >
        <option value="">All statuses</option>
        {statusOptions.map((s) => (
          <option key={s} value={s}>
            {s.charAt(0).toUpperCase() + s.slice(1)}
          </option>
        ))}
      </select>

      {/* Visibility — instant, admin only */}
      {showVisibility && (
        <select
          name="visibility"
          value={currentVisibility}
          onChange={(e) => applyNow({ visibility: e.target.value })}
          className={selectClass}
          aria-label="Filter by visibility"
        >
          <option value="">All visibility</option>
          <option value="public">Public</option>
          <option value="private">Private</option>
        </select>
      )}

      {/* Clear — always visible; disabled + muted when no filters are active */}
      <Button
        type="button"
        variant="ghost"
        size="sm"
        className="shrink-0 gap-1.5"
        onClick={() => {
          setQ('')
          setNamespace('')
          navigate(pathname, { replace: true })
        }}
        aria-label="Clear all filters"
        disabled={!hasFilters}
        aria-disabled={!hasFilters}
      >
        <X className="h-3.5 w-3.5" aria-hidden="true" />
        Clear
      </Button>
    </form>
  )
}
