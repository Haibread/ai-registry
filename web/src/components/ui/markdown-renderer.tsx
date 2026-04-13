/**
 * MarkdownRenderer — renders Markdown content with GitHub-flavored support.
 *
 * Uses react-markdown + remark-gfm for tables, strikethrough, task lists, etc.
 * Styled with Tailwind prose classes.
 */

import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { cn } from '@/lib/utils'

interface MarkdownRendererProps {
  content: string
  className?: string
}

export function MarkdownRenderer({ content, className }: MarkdownRendererProps) {
  if (!content) return null

  return (
    <div
      className={cn(
        'prose prose-sm dark:prose-invert max-w-none',
        // Override prose defaults for tighter spacing
        'prose-headings:mt-4 prose-headings:mb-2',
        'prose-p:my-2',
        'prose-pre:bg-muted prose-pre:text-foreground',
        'prose-code:before:content-none prose-code:after:content-none',
        'prose-code:bg-muted prose-code:px-1 prose-code:py-0.5 prose-code:rounded prose-code:text-xs',
        'prose-a:text-primary',
        className,
      )}
    >
      <ReactMarkdown remarkPlugins={[remarkGfm]}>{content}</ReactMarkdown>
    </div>
  )
}
