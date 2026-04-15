/**
 * StickyDetailHeader — compact header that appears when the user scrolls
 * past the main title on a detail page.
 *
 * Uses IntersectionObserver on a sentinel element (the title).
 */

import { useState, useEffect } from 'react'
import { Badge } from '@/components/ui/badge'
import { CopyButton } from '@/components/ui/copy-button'
import { ResourceIcon } from '@/components/ui/resource-icon'
import { cn } from '@/lib/utils'

interface StickyDetailHeaderProps {
  type: 'mcp-server' | 'agent'
  name: string
  version?: string
  identifier: string
  /** Ref to the title element — when it scrolls out of view, the sticky header shows */
  titleRef: React.RefObject<HTMLElement | null>
}

export function StickyDetailHeader({
  type,
  name,
  version,
  identifier,
  titleRef,
}: StickyDetailHeaderProps) {
  const [visible, setVisible] = useState(false)

  useEffect(() => {
    const el = titleRef.current
    if (!el) return

    const observer = new IntersectionObserver(
      ([entry]) => {
        setVisible(!entry.isIntersecting)
      },
      { threshold: 0, rootMargin: '-64px 0px 0px 0px' }, // 64px = header height
    )

    observer.observe(el)
    return () => observer.disconnect()
  }, [titleRef])

  return (
    <div
      className={cn(
        'fixed top-14 left-0 right-0 z-40 border-b bg-background/95 backdrop-blur-sm transition-all duration-200',
        visible ? 'translate-y-0 opacity-100' : '-translate-y-full opacity-0 pointer-events-none',
      )}
    >
      <div className="container flex items-center gap-3 h-10 max-w-3xl">
        <ResourceIcon type={type} className="h-4 w-4 text-muted-foreground shrink-0" />
        <span className="text-sm font-semibold truncate">{name}</span>
        {version && (
          <Badge variant="outline" className="text-[10px] font-mono shrink-0">
            v{version}
          </Badge>
        )}
        <div className="ml-auto flex items-center gap-2 shrink-0">
          <CopyButton value={identifier} label="Copy identifier" />
        </div>
      </div>
    </div>
  )
}
