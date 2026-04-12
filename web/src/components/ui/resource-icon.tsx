import { Plug, Bot, Building2, Zap, MessageSquare } from "lucide-react"
import { cn } from "@/lib/utils"

export type ResourceType = "mcp-server" | "agent" | "publisher" | "skill" | "prompt"

const iconMap = {
  "mcp-server": Plug,
  agent: Bot,
  publisher: Building2,
  skill: Zap,
  prompt: MessageSquare,
} as const

interface ResourceIconProps {
  type: ResourceType
  className?: string
}

/**
 * Renders a consistent icon for a given resource type.
 * Use in navigation, cards, breadcrumbs, and search results.
 */
export function ResourceIcon({ type, className }: ResourceIconProps) {
  const Icon = iconMap[type]
  return <Icon className={cn("h-4 w-4", className)} aria-hidden="true" />
}
