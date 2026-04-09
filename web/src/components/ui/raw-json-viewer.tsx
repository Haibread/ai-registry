import { useState } from "react"
import { ChevronDown, ChevronRight, Copy, Check } from "lucide-react"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"

interface RawJsonViewerProps {
  data: unknown
  title?: string
  defaultOpen?: boolean
}

export function RawJsonViewer({ data, title = "Raw JSON", defaultOpen = false }: RawJsonViewerProps) {
  const [open, setOpen] = useState(defaultOpen)
  const [copied, setCopied] = useState(false)

  const json = JSON.stringify(data, null, 2)

  async function copy() {
    await navigator.clipboard.writeText(json)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  return (
    <div className="border rounded-md overflow-hidden">
      <div
        role="button"
        tabIndex={0}
        onClick={() => setOpen((v) => !v)}
        onKeyDown={(e) => { if (e.key === "Enter" || e.key === " ") setOpen((v) => !v) }}
        className="w-full flex items-center justify-between px-4 py-3 text-sm font-medium hover:bg-muted/50 transition-colors cursor-pointer select-none"
      >
        <span className="flex items-center gap-2">
          {open ? (
            <ChevronDown className="h-4 w-4 text-muted-foreground" />
          ) : (
            <ChevronRight className="h-4 w-4 text-muted-foreground" />
          )}
          {title}
        </span>
        {open && (
          <Button
            variant="ghost"
            size="sm"
            className="h-7 px-2 text-xs"
            onClick={(e) => { e.stopPropagation(); copy() }}
            aria-label={copied ? "Copied to clipboard" : "Copy JSON to clipboard"}
          >
            {copied ? <Check className="h-3.5 w-3.5" aria-hidden="true" /> : <Copy className="h-3.5 w-3.5" aria-hidden="true" />}
            <span className="ml-1">{copied ? "Copied" : "Copy"}</span>
          </Button>
        )}
      </div>

      {open && (
        <pre className={cn(
          "bg-muted/40 text-xs font-mono p-4 overflow-auto max-h-[480px]",
          "border-t leading-relaxed"
        )}>
          {json}
        </pre>
      )}
    </div>
  )
}
