import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MarkdownRenderer } from './markdown-renderer'

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
})
