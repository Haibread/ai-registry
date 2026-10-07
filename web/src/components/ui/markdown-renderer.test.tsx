import { describe, it, expect, vi, beforeEach } from 'vitest'
import { act, render, screen, waitFor } from '@testing-library/react'
import { MarkdownRenderer } from './markdown-renderer'

const mermaid = vi.hoisted(() => ({
  initialize: vi.fn(),
  render: vi.fn(),
}))

vi.mock('mermaid', () => ({ default: mermaid }))

describe('MarkdownRenderer', () => {
  it('renders markdown headings', () => {
    render(<MarkdownRenderer content="# Hello World" />)
    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Hello World')
  })

  it('renders paragraphs', () => {
    render(<MarkdownRenderer content="This is a paragraph." />)
    expect(screen.getByText('This is a paragraph.')).toBeInTheDocument()
  })

  it('renders links', () => {
    render(<MarkdownRenderer content="[Example](https://example.com)" />)
    const link = screen.getByRole('link', { name: 'Example' })
    expect(link).toHaveAttribute('href', 'https://example.com')
  })

  it('renders inline code', () => {
    render(<MarkdownRenderer content="Use `npm install` to install." />)
    expect(screen.getByText('npm install')).toBeInTheDocument()
  })

  it('renders GFM tables', () => {
    const md = `
| Name | Value |
|------|-------|
| foo  | bar   |
`
    render(<MarkdownRenderer content={md} />)
    expect(screen.getByRole('table')).toBeInTheDocument()
    expect(screen.getByText('foo')).toBeInTheDocument()
    expect(screen.getByText('bar')).toBeInTheDocument()
  })

  it('renders raw HTML as inert text instead of elements', () => {
    const { container } = render(
      <MarkdownRenderer content={'<script>alert(1)</script>\n\nhi <img src=x onerror="alert(1)"> there'} />,
    )
    expect(container.querySelector('script')).toBeNull()
    expect(container.querySelector('img')).toBeNull()
    expect(container).toHaveTextContent('<script>alert(1)</script>')
  })

  it.each([
    ['inline link', '[click](javascript:alert(1))', 'click'],
    ['mixed-case scheme', '[click](JaVaScRiPt:alert(1))', 'click'],
    ['autolink', '<javascript:alert(1)>', 'javascript:alert(1)'],
  ])('neutralises javascript: URLs in an %s', (_, md, name) => {
    render(<MarkdownRenderer content={md} />)
    expect(screen.getByText(name).closest('a')).toHaveAttribute('href', '')
  })

  it('keeps relative links', () => {
    render(<MarkdownRenderer content="[docs](./docs/setup.md)" />)
    expect(screen.getByRole('link', { name: 'docs' })).toHaveAttribute('href', './docs/setup.md')
  })

  it('renders nothing when content is empty', () => {
    const { container } = render(<MarkdownRenderer content="" />)
    expect(container.innerHTML).toBe('')
  })

  it('applies prose classes', () => {
    const { container } = render(<MarkdownRenderer content="Hello" />)
    const wrapper = container.firstChild as HTMLElement
    expect(wrapper.className).toContain('prose')
  })

  it('accepts custom className', () => {
    const { container } = render(<MarkdownRenderer content="Hello" className="my-custom" />)
    const wrapper = container.firstChild as HTMLElement
    expect(wrapper.className).toContain('my-custom')
  })

  describe('mermaid blocks', () => {
    const diagram = '```mermaid\ngraph TD\n  A-->B\n```'

    beforeEach(() => {
      mermaid.initialize.mockReset()
      mermaid.render.mockReset()
      mermaid.render.mockImplementation(async (id: string) => ({ svg: `<svg id="${id}" style="max-width: 640px;"><g></g></svg>` }))
      document.documentElement.classList.remove('dark')
    })

    it('renders the diagram SVG in strict mode', async () => {
      const { container } = render(<MarkdownRenderer content={diagram} />)
      await waitFor(() => expect(container.querySelector('[data-slot="mermaid-diagram"] svg')).not.toBeNull())
      expect(mermaid.render).toHaveBeenCalledWith(expect.stringMatching(/^mermaid-\d+$/), 'graph TD\n  A-->B')
      expect(mermaid.initialize).toHaveBeenCalledWith(
        expect.objectContaining({
          startOnLoad: false,
          securityLevel: 'strict',
          htmlLabels: false,
          suppressErrorRendering: true,
          theme: 'default',
          secure: expect.arrayContaining(['securityLevel', 'htmlLabels', 'themeCSS']),
        }),
      )
      expect(container.querySelector('pre')).toBeNull()
      const svg = container.querySelector('svg') as SVGElement
      expect(svg.style.minWidth).toBe('640px')
    })

    it('gives every diagram its own id', async () => {
      const { container } = render(<MarkdownRenderer content={`${diagram}\n\n${diagram}`} />)
      await waitFor(() => expect(container.querySelectorAll('[data-slot="mermaid-diagram"] svg')).toHaveLength(2))
      const ids = mermaid.render.mock.calls.map(([id]) => id)
      expect(new Set(ids).size).toBe(ids.length)
    })

    it('leaves other code blocks untouched', () => {
      const { container } = render(<MarkdownRenderer content={'```js\nconst a = 1\n```'} />)
      const code = container.querySelector('pre > code')
      expect(code).toHaveClass('language-js')
      expect(code).toHaveTextContent('const a = 1')
      expect(mermaid.render).not.toHaveBeenCalled()
    })

    it('falls back to the source when the diagram is invalid', async () => {
      mermaid.render.mockRejectedValue(new Error('Parse error'))
      const { container } = render(<MarkdownRenderer content={'```mermaid\nnot a diagram\n```'} />)
      expect(await screen.findByText('This Mermaid diagram could not be rendered.')).toBeInTheDocument()
      expect(container.querySelector('pre > code.language-mermaid')).toHaveTextContent('not a diagram')
      expect(container.querySelector('svg')).toBeNull()
    })

    it('re-renders with the dark theme when the app switches to dark mode', async () => {
      render(<MarkdownRenderer content={diagram} />)
      await waitFor(() => expect(mermaid.render).toHaveBeenCalledTimes(1))
      act(() => document.documentElement.classList.add('dark'))
      await waitFor(() => expect(mermaid.initialize).toHaveBeenLastCalledWith(expect.objectContaining({ theme: 'dark' })))
      await waitFor(() => expect(mermaid.render).toHaveBeenCalledTimes(2))
      document.documentElement.classList.remove('dark')
    })
  })
})
