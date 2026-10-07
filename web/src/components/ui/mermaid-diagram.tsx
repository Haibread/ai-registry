import { useEffect, useLayoutEffect, useRef, useState, useSyncExternalStore } from 'react'
import type { Mermaid, MermaidConfig } from 'mermaid'

type Theme = 'light' | 'dark'

type RenderResult = { svg: string } | { error: true }

interface MermaidDiagramProps {
  source: string
}

let mermaidModule: Promise<Mermaid> | null = null

function loadMermaid(): Promise<Mermaid> {
  mermaidModule ??= import('mermaid').then(
    (m) => m.default,
    (err: unknown) => {
      mermaidModule = null
      throw err
    },
  )
  return mermaidModule
}

// The diagram source is publisher-written, so it is untrusted. `strict` makes
// mermaid escape label text, drop click callbacks and run its SVG through
// DOMPurify. `secure` lists the keys a `%%{init}%%` directive or frontmatter
// `config:` may not override (nested occurrences included): mermaid's own
// defaults, plus `htmlLabels` (HTML in foreignObject), `themeCSS` (arbitrary
// CSS, e.g. `@import url(...)` beacons) and the theme, which follows the app.
function mermaidConfig(theme: Theme): MermaidConfig {
  return {
    startOnLoad: false,
    securityLevel: 'strict',
    htmlLabels: false,
    // Without it a syntax error leaves mermaid's error SVG in <body>.
    suppressErrorRendering: true,
    theme: theme === 'dark' ? 'dark' : 'default',
    secure: [
      'secure',
      'securityLevel',
      'startOnLoad',
      'maxTextSize',
      'suppressErrorRendering',
      'maxEdges',
      'htmlLabels',
      'themeCSS',
      'theme',
      'darkMode',
    ],
  }
}

let renderSeq = 0

async function renderDiagram(source: string, theme: Theme): Promise<string> {
  const mermaid = await loadMermaid()
  mermaid.initialize(mermaidConfig(theme))
  const id = `mermaid-${++renderSeq}`
  try {
    const { svg } = await mermaid.render(id, source)
    return svg
  } finally {
    document.getElementById(`d${id}`)?.remove()
  }
}

function subscribeToTheme(onChange: () => void): () => void {
  const observer = new MutationObserver(onChange)
  observer.observe(document.documentElement, { attributes: true, attributeFilter: ['class'] })
  return () => observer.disconnect()
}

function currentTheme(): Theme {
  return document.documentElement.classList.contains('dark') ? 'dark' : 'light'
}

export function MermaidDiagram({ source }: MermaidDiagramProps) {
  const theme = useSyncExternalStore(subscribeToTheme, currentTheme)
  const [result, setResult] = useState<RenderResult | null>(null)
  const container = useRef<HTMLDivElement>(null)

  useEffect(() => {
    let cancelled = false
    renderDiagram(source, theme).then(
      (svg) => !cancelled && setResult({ svg }),
      () => !cancelled && setResult({ error: true }),
    )
    return () => {
      cancelled = true
    }
  }, [source, theme])

  const svg = result && 'svg' in result ? result.svg : null

  // Mermaid sizes most diagrams `width="100%"` capped at their natural width,
  // which shrinks a wide diagram to unreadable on a phone. Pinning the minimum
  // to that natural width makes the container scroll instead.
  useLayoutEffect(() => {
    const el = container.current?.querySelector('svg')
    if (el?.style.maxWidth) el.style.minWidth = el.style.maxWidth
  }, [svg])

  if (result && 'error' in result) {
    return (
      <div className="my-4">
        <pre className="my-0">
          <code className="language-mermaid">{source}</code>
        </pre>
        <p className="mt-1 mb-0 text-xs text-muted-foreground">This Mermaid diagram could not be rendered.</p>
      </div>
    )
  }

  if (svg === null) {
    return (
      <div aria-busy="true" className="not-prose my-4 h-24 animate-pulse rounded-md bg-muted">
        <span className="sr-only">Rendering diagram</span>
      </div>
    )
  }

  return (
    <div
      ref={container}
      data-slot="mermaid-diagram"
      className="not-prose my-4 overflow-x-auto"
      // Mermaid's own output, sanitized by DOMPurify under securityLevel strict.
      dangerouslySetInnerHTML={{ __html: svg }}
    />
  )
}
