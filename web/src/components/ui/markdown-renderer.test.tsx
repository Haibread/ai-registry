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
