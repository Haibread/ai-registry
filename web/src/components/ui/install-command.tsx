import { cn } from "@/lib/utils"
import { CopyButton } from "@/components/ui/copy-button"

interface InstallCommandProps {
  command: string
  className?: string
  /** Optional callback fired after a successful copy */
  onCopy?: () => void
}

export function InstallCommand({ command, className, onCopy }: InstallCommandProps) {
  return (
    <div className={cn("flex items-center gap-2 bg-muted rounded-md px-3 py-2 group", className)}>
      <code className="text-xs font-mono flex-1 overflow-x-auto whitespace-nowrap">{command}</code>
      <CopyButton value={command} label="Copy command" onCopy={onCopy} />
    </div>
  )
}
