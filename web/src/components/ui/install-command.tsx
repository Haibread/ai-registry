"use client"

import { useState } from "react"
import { Copy, Check } from "lucide-react"
import { cn } from "@/lib/utils"

interface InstallCommandProps {
  command: string
  className?: string
}

export function InstallCommand({ command, className }: InstallCommandProps) {
  const [copied, setCopied] = useState(false)

  async function copy() {
    await navigator.clipboard.writeText(command)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  return (
    <div className={cn("flex items-center gap-2 bg-muted rounded-md px-3 py-2 group", className)}>
      <code className="text-xs font-mono flex-1 overflow-x-auto whitespace-nowrap">{command}</code>
      <button
        type="button"
        onClick={copy}
        className="shrink-0 text-muted-foreground hover:text-foreground transition-colors"
        aria-label="Copy command"
      >
        {copied ? <Check className="h-3.5 w-3.5" /> : <Copy className="h-3.5 w-3.5" />}
      </button>
    </div>
  )
}
