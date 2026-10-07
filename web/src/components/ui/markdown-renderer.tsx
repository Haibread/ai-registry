/**
 * MarkdownRenderer — renders Markdown content with GitHub-flavored support.
 *
 * Uses react-markdown + remark-gfm for tables, strikethrough, task lists, etc.
 * ```mermaid fenced blocks render as diagrams. Styled with Tailwind prose classes.
 */

import ReactMarkdown, { type Components, type ExtraProps } from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { MermaidDiagram } from '@/components/ui/mermaid-diagram'
import { cn } from '@/lib/utils'

type HastElement = NonNullable<ExtraProps['node']>

function mermaidSource(pre: HastElement | undefined): string | null {
  const code = pre?.children[0]
  if (code?.type !== 'element' || code.tagName !== 'code') return null
  const classes = code.properties.className
  if (!Array.isArray(classes) || !classes.includes('language-mermaid')) return null
  return code.children
    .map((child) => (child.type === 'text' ? child.value : ''))
    .join('')
    .trimEnd()
}

const components: Components = {
  pre({ node, ...props }) {
    const source = mermaidSource(node)
    return source === null ? <pre {...props} /> : <MermaidDiagram source={source} />
  },
}

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
      <ReactMarkdown remarkPlugins={[remarkGfm]} components={components}>{content}</ReactMarkdown>
    </div>
  )
}
