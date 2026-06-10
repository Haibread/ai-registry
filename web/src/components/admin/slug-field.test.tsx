import { describe, it, expect } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { SlugField } from './slug-field'
import { slugError, SLUG_MAX_LENGTH } from '@/lib/slug'

describe('slugError', () => {
  it('accepts valid slugs', () => {
    expect(slugError('my-server')).toBeNull()
    expect(slugError('a1-b2-c3')).toBeNull()
    expect(slugError('')).toBeNull() // emptiness is `required`'s job
  })

  it('rejects uppercase, spaces, and punctuation', () => {
    expect(slugError('Bad Slug!!')).toMatch(/lowercase/i)
    expect(slugError('MyServer')).toMatch(/lowercase/i)
    expect(slugError('a_b')).toMatch(/lowercase/i)
  })

  it('rejects slugs over the max length', () => {
    expect(slugError('a'.repeat(SLUG_MAX_LENGTH))).toBeNull()
    expect(slugError('a'.repeat(SLUG_MAX_LENGTH + 1))).toMatch(/63/)
  })
})

describe('SlugField', () => {
  it('renders the hint when pristine', () => {
    render(<SlugField placeholder="my-thing" />)
    expect(screen.getByText(/lowercase letters, numbers, and hyphens only/i)).toBeInTheDocument()
    expect(screen.getByLabelText(/slug/i)).not.toHaveAttribute('aria-invalid')
  })

  it('shows an inline error with aria-invalid on blur for a bad slug', () => {
    render(<SlugField placeholder="my-thing" />)
    const input = screen.getByLabelText(/slug/i)
    fireEvent.change(input, { target: { value: 'Bad Slug!!' } })
    fireEvent.blur(input)
    const error = screen.getByRole('alert')
    expect(error).toHaveTextContent(/lowercase/i)
    expect(input).toHaveAttribute('aria-invalid', 'true')
    expect(input).toHaveAttribute('aria-describedby', 'slug-error')
  })

  it('clears the error as the user fixes the value', () => {
    render(<SlugField placeholder="my-thing" />)
    const input = screen.getByLabelText(/slug/i)
    fireEvent.change(input, { target: { value: 'Bad Slug' } })
    fireEvent.blur(input)
    expect(screen.getByRole('alert')).toBeInTheDocument()
    fireEvent.change(input, { target: { value: 'good-slug' } })
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    expect(input).not.toHaveAttribute('aria-invalid')
  })

  it('uses a v-flag-compatible HTML pattern (the old one compiled to nothing)', () => {
    render(<SlugField placeholder="my-thing" />)
    const input = screen.getByLabelText(/slug/i) as HTMLInputElement
    // The regression this guards: `^[a-z0-9-]+` throws under the `v` flag, so
    // browsers silently skipped validation. The attribute must compile.
    expect(() => new RegExp(`^(?:${input.pattern})$`, 'v')).not.toThrow()
    fireEvent.change(input, { target: { value: 'Bad Slug!!' } })
    expect(input.checkValidity()).toBe(false)
    fireEvent.change(input, { target: { value: 'good-slug' } })
    expect(input.checkValidity()).toBe(true)
  })
})
