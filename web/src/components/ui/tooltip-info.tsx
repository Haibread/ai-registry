import * as TooltipPrimitive from "@radix-ui/react-tooltip"
import { Info } from "lucide-react"
import { cn } from "@/lib/utils"

interface TooltipInfoProps {
  content: string
  className?: string
  iconClassName?: string
}

/**
 * A small info icon that shows an explanatory tooltip on hover.
 * Use next to technical fields like "runtime", "transport", etc.
 */
export function TooltipInfo({ content, className, iconClassName }: TooltipInfoProps) {
  return (
    <TooltipPrimitive.Provider delayDuration={200}>
      <TooltipPrimitive.Root>
        <TooltipPrimitive.Trigger asChild>
          <button
            type="button"
            className={cn(
              "inline-flex items-center text-muted-foreground/60 hover:text-muted-foreground transition-colors",
              className,
            )}
            aria-label="More information"
          >
            <Info className={cn("h-3.5 w-3.5", iconClassName)} />
          </button>
        </TooltipPrimitive.Trigger>
        <TooltipPrimitive.Portal>
          <TooltipPrimitive.Content
            side="top"
            sideOffset={4}
            className="z-50 max-w-xs rounded-md border bg-popover px-3 py-2 text-xs text-popover-foreground shadow-md animate-in fade-in-0 zoom-in-95 data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=closed]:zoom-out-95"
          >
            {content}
            <TooltipPrimitive.Arrow className="fill-popover" />
          </TooltipPrimitive.Content>
        </TooltipPrimitive.Portal>
      </TooltipPrimitive.Root>
    </TooltipPrimitive.Provider>
  )
}
