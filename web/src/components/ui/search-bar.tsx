/**
 * SearchBar — unified search input with live dropdown results.
 *
 * On type (debounced 300ms): parallel queries to MCP servers and agents list
 * endpoints. Results appear in a dropdown grouped by type.
 * Click → navigate to detail page. Enter → navigate to /explore with q param.
 *
 * The "hero" variant is large and centered (used on the home page). The
 * "compact" variant is smaller and flex-fills its container (used in the
 * global header so cross-type search is one click away from every page).
 */

import { useState, useCallback, useRef, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { Search } from 'lucide-react'
import { Input } from '@/components/ui/input'
import { ResourceIcon } from '@/components/ui/resource-icon'
import { getPublicClient } from '@/lib/api-client'

const DEBOUNCE_MS = 300

interface SearchBarProps {
  variant?: 'hero' | 'compact'
}

export function SearchBar({ variant = 'hero' }: SearchBarProps = {}) {
  const navigate = useNavigate()
  const [query, setQuery] = useState('')
  const [debouncedQuery, setDebouncedQuery] = useState('')
  const [open, setOpen] = useState(false)
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const containerRef = useRef<HTMLDivElement>(null)

  const api = getPublicClient()

  // Debounce the search query
  const handleChange = useCallback((value: string) => {
    setQuery(value)
    if (debounceRef.current) clearTimeout(debounceRef.current)
    if (!value.trim()) {
      setDebouncedQuery('')
      setOpen(false)
      return
    }
    debounceRef.current = setTimeout(() => {
      setDebouncedQuery(value.trim())
      setOpen(true)
    }, DEBOUNCE_MS)
  }, [])

  useEffect(() => () => { if (debounceRef.current) clearTimeout(debounceRef.current) }, [])

  // Parallel queries
  const { data: mcpData } = useQuery({
    queryKey: ['search-mcp', debouncedQuery],
    queryFn: () => api.GET('/api/v1/mcp/servers', {
      params: { query: { q: debouncedQuery, limit: 5 } },
    }).then(r => r.data),
    enabled: debouncedQuery.length > 0,
  })

  const { data: agentData } = useQuery({
    queryKey: ['search-agents', debouncedQuery],
    queryFn: () => api.GET('/api/v1/agents', {
      params: { query: { q: debouncedQuery, limit: 5 } },
    }).then(r => r.data),
    enabled: debouncedQuery.length > 0,
  })

  const mcpResults = mcpData?.items ?? []
  const agentResults = agentData?.items ?? []
  const hasResults = mcpResults.length > 0 || agentResults.length > 0

  // Close dropdown on outside click
  useEffect(() => {
    const handler = (e: MouseEvent) => {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
        setOpen(false)
      }
    }
    document.addEventListener('mousedown', handler)
    return () => document.removeEventListener('mousedown', handler)
  }, [])

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (query.trim()) {
      setOpen(false)
      navigate(`/explore?q=${encodeURIComponent(query.trim())}`)
    }
  }

  const isCompact = variant === 'compact'
  const containerCls = isCompact
    ? 'relative w-full max-w-sm'
    : 'relative w-full max-w-lg mx-auto'
  const iconCls = isCompact
    ? 'absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground pointer-events-none'
    : 'absolute left-4 top-1/2 -translate-y-1/2 h-5 w-5 text-muted-foreground pointer-events-none'
  const inputCls = isCompact
    ? 'pl-9 h-9 text-sm rounded-md'
    : 'pl-12 h-12 text-base rounded-xl shadow-sm'
  const placeholder = isCompact
    ? 'Search…'
    : 'Search MCP servers and agents...'

  return (
    <div ref={containerRef} className={containerCls}>
      <form onSubmit={handleSubmit}>
        <div className="relative">
          <Search className={iconCls} />
          <Input
            value={query}
            onChange={(e) => handleChange(e.target.value)}
            onFocus={() => { if (debouncedQuery && hasResults) setOpen(true) }}
            placeholder={placeholder}
            className={inputCls}
            autoComplete="off"
            aria-label="Search registry"
          />
        </div>
      </form>

      {/* Dropdown */}
      {open && debouncedQuery && hasResults && (
        <div className="absolute z-50 mt-1 w-full rounded-lg border bg-popover shadow-lg overflow-hidden">
          {mcpResults.length > 0 && (
            <div>
              <div className="px-3 py-1.5 text-xs font-medium text-muted-foreground uppercase tracking-wide bg-muted/50">
                MCP Servers
              </div>
              {mcpResults.map((s) => (
                <button
                  key={s.id}
                  type="button"
                  className="w-full flex items-center gap-3 px-3 py-2 text-left hover:bg-accent transition-colors"
                  onClick={() => {
                    setOpen(false)
                    navigate(`/mcp/${s.namespace}/${s.slug}`)
                  }}
                >
                  <ResourceIcon type="mcp-server" className="h-4 w-4 shrink-0 text-muted-foreground" />
                  <div className="min-w-0">
                    <p className="text-sm font-medium truncate">{s.name}</p>
                    <p className="text-xs text-muted-foreground font-mono truncate">{s.namespace}/{s.slug}</p>
                  </div>
                </button>
              ))}
            </div>
          )}
          {agentResults.length > 0 && (
            <div>
              <div className="px-3 py-1.5 text-xs font-medium text-muted-foreground uppercase tracking-wide bg-muted/50">
                Agents
              </div>
              {agentResults.map((a) => (
                <button
                  key={a.id}
                  type="button"
                  className="w-full flex items-center gap-3 px-3 py-2 text-left hover:bg-accent transition-colors"
                  onClick={() => {
                    setOpen(false)
                    navigate(`/agents/${a.namespace}/${a.slug}`)
                  }}
                >
                  <ResourceIcon type="agent" className="h-4 w-4 shrink-0 text-muted-foreground" />
                  <div className="min-w-0">
                    <p className="text-sm font-medium truncate">{a.name}</p>
                    <p className="text-xs text-muted-foreground font-mono truncate">{a.namespace}/{a.slug}</p>
                  </div>
                </button>
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  )
}
