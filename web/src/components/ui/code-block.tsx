import { CopyButton } from '@/components/ui/copy-button'

interface CodeBlockProps {
  value: string
  copyLabel?: string
}

export function CodeBlock({ value, copyLabel }: CodeBlockProps) {
  return (
    <div className="relative rounded-md bg-muted overflow-hidden">
      <div className="absolute top-2 right-2 z-10">
        <CopyButton value={value} label={copyLabel} />
      </div>
      <pre className="p-3 pr-12 text-xs font-mono overflow-x-auto whitespace-pre">{value}</pre>
    </div>
  )
}
