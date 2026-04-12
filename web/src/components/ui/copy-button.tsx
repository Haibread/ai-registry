import { useState, useCallback } from "react"
import { Copy, Check } from "lucide-react"
import { cn } from "@/lib/utils"

interface CopyButtonProps {
  value: string
  className?: string
  iconSize?: string
  label?: string
}

/**
 * A standalone copy-to-clipboard button. Shows a check icon for 2 seconds
 * after copying.
 *
 * Use this wherever a value needs to be copied: identifiers, URLs, code
 * snippets, install commands, etc.
 */
export function CopyButton({
  value,
  className,
  iconSize = "h-3.5 w-3.5",
  label = "Copy to clipboard",
}: CopyButtonProps) {
  const [copied, setCopied] = useState(false)

  const copy = useCallback(async () => {
    await navigator.clipboard.writeText(value)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }, [value])

  return (
    <button
      type="button"
      onClick={copy}
      className={cn(
        "shrink-0 text-muted-foreground hover:text-foreground transition-colors",
        className,
      )}
      aria-label={label}
    >
      {copied ? (
        <Check className={cn(iconSize, "text-green-600")} />
      ) : (
        <Copy className={iconSize} />
      )}
    </button>
  )
}
