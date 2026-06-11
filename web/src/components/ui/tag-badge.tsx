/**
 * TagBadge — chip for an instance tag (the Server-Admin-curated vocabulary).
 *
 * Renders the tag's display name in its configured color. When the slug
 * cannot be resolved against the vocabulary (e.g. the definition was
 * deactivated and later the list failed to load), the slug itself renders in
 * the neutral gray so frozen versions never lose their tags.
 */

import type { InstanceTag } from '@/lib/use-instance-tags'
import { cn } from '@/lib/utils'

/** One entry per `color` enum value of the InstanceTag schema. */
const tagColorClasses: Record<string, string> = {
  gray: 'bg-gray-100 text-gray-800 dark:bg-gray-800 dark:text-gray-200',
  red: 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200',
  orange: 'bg-orange-100 text-orange-800 dark:bg-orange-900 dark:text-orange-200',
  yellow: 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200',
  green: 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200',
  teal: 'bg-teal-100 text-teal-800 dark:bg-teal-900 dark:text-teal-200',
  blue: 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200',
  indigo: 'bg-indigo-100 text-indigo-800 dark:bg-indigo-900 dark:text-indigo-200',
  purple: 'bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-200',
  pink: 'bg-pink-100 text-pink-800 dark:bg-pink-900 dark:text-pink-200',
}

interface TagBadgeProps {
  /** The tag slug as carried by the entry/version. */
  slug: string
  /** The resolved definition, when the vocabulary lookup found one. */
  tag?: InstanceTag | undefined
  className?: string
}

export function TagBadge({ slug, tag, className }: TagBadgeProps) {
  const color = tag?.color && tagColorClasses[tag.color] ? tag.color : 'gray'
  return (
    <span
      title={tag?.description || undefined}
      className={cn(
        'inline-flex items-center rounded-full border border-transparent px-2.5 py-0.5 text-xs font-semibold',
        tagColorClasses[color],
        className,
      )}
    >
      {tag?.name ?? slug}
    </span>
  )
}
