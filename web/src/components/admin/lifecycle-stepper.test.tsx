import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { LifecycleStepper } from './lifecycle-stepper'

describe('LifecycleStepper', () => {
  it('renders all stages', () => {
    render(<LifecycleStepper currentStatus="draft" />)
    expect(screen.getByText('Draft')).toBeInTheDocument()
    expect(screen.getByText('Published')).toBeInTheDocument()
    expect(screen.getByText('Deprecated')).toBeInTheDocument()
    expect(screen.getByText('Deleted')).toBeInTheDocument()
  })

  it('highlights the current stage', () => {
    render(<LifecycleStepper currentStatus="published" />)
    const published = screen.getByText('Published').closest('button')!
    expect(published.className).toContain('bg-primary/10')
  })

  it('makes published clickable from draft', () => {
    const onTransition = vi.fn()
    render(<LifecycleStepper currentStatus="draft" onTransition={onTransition} />)
    const button = screen.getByText('Published').closest('button')!
    expect(button).not.toBeDisabled()
    fireEvent.click(button)
    expect(onTransition).toHaveBeenCalledWith('published')
  })

  it('makes deprecated clickable from published', () => {
    const onTransition = vi.fn()
    render(<LifecycleStepper currentStatus="published" onTransition={onTransition} />)
    const button = screen.getByText('Deprecated').closest('button')!
    expect(button).not.toBeDisabled()
    fireEvent.click(button)
    expect(onTransition).toHaveBeenCalledWith('deprecated')
  })

  it('disables non-transition targets', () => {
    render(<LifecycleStepper currentStatus="draft" />)
    const deprecated = screen.getByText('Deprecated').closest('button')!
    expect(deprecated).toBeDisabled()
  })

  it('disables deleted target by default', () => {
    render(<LifecycleStepper currentStatus="published" />)
    const deleted = screen.getByText('Deleted').closest('button')!
    expect(deleted).toBeDisabled()
  })

  it('accepts custom allowed transitions', () => {
    const onTransition = vi.fn()
    render(
      <LifecycleStepper
        currentStatus="published"
        allowedTransitions={['draft']}
        onTransition={onTransition}
      />,
    )
    const draft = screen.getByText('Draft').closest('button')!
    expect(draft).not.toBeDisabled()
    fireEvent.click(draft)
    expect(onTransition).toHaveBeenCalledWith('draft')
  })

  it('does nothing when clicking current stage', () => {
    const onTransition = vi.fn()
    render(<LifecycleStepper currentStatus="published" onTransition={onTransition} />)
    const button = screen.getByText('Published').closest('button')!
    fireEvent.click(button)
    expect(onTransition).not.toHaveBeenCalled()
  })

  it('allows transition from deprecated back to published', () => {
    const onTransition = vi.fn()
    render(<LifecycleStepper currentStatus="deprecated" onTransition={onTransition} />)
    const button = screen.getByText('Published').closest('button')!
    expect(button).not.toBeDisabled()
    fireEvent.click(button)
    expect(onTransition).toHaveBeenCalledWith('published')
  })
})
