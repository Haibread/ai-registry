import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { SegmentedControl } from './segmented-control'

const OPTIONS = [
  { value: 'a', label: 'Alpha' },
  { value: 'b', label: 'Beta' },
] as const

describe('SegmentedControl', () => {
  it('exposes a labelled group with the selected option pressed', () => {
    render(<SegmentedControl label="Pick" options={OPTIONS} value="b" onChange={vi.fn()} />)
    expect(screen.getByRole('group', { name: 'Pick' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Alpha' })).toHaveAttribute('aria-pressed', 'false')
    expect(screen.getByRole('button', { name: 'Beta' })).toHaveAttribute('aria-pressed', 'true')
  })

  it('reports the clicked option', async () => {
    const onChange = vi.fn()
    render(<SegmentedControl label="Pick" options={OPTIONS} value="a" onChange={onChange} />)
    await userEvent.setup().click(screen.getByRole('button', { name: 'Beta' }))
    expect(onChange).toHaveBeenCalledWith('b')
  })
})
