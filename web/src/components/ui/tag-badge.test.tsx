/**
 * tag-badge.test.tsx
 *
 * TagBadge resolves an instance-tag slug against the vocabulary: a resolved
 * tag shows its display name in its configured color (description as the
 * hover title); an unresolved slug must still render — published versions
 * keep carrying slugs whose definitions were deactivated or failed to load —
 * falling back to the slug text in the neutral gray.
 */

import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { TagBadge } from './tag-badge'
import type { InstanceTag } from '@/lib/use-instance-tags'

const tag: InstanceTag = {
  slug: 'early-access',
  name: 'Early Access',
  description: 'Pre-GA functionality',
  color: 'blue',
  active: true,
  created_at: '2026-06-01T10:00:00Z',
  updated_at: '2026-06-01T10:00:00Z',
}

describe('TagBadge', () => {
  it('renders the resolved display name with the description as title', () => {
    render(<TagBadge slug="early-access" tag={tag} />)
    const badge = screen.getByText('Early Access')
    expect(badge).toHaveAttribute('title', 'Pre-GA functionality')
    expect(badge.className).toContain('bg-blue-100')
  })

  it('falls back to the slug when the tag is unresolved', () => {
    render(<TagBadge slug="early-access" />)
    const badge = screen.getByText('early-access')
    expect(badge).not.toHaveAttribute('title')
    expect(badge.className).toContain('bg-gray-100')
  })

  it('applies the gray fallback for an unknown color', () => {
    const odd = { ...tag, color: 'magenta' as InstanceTag['color'] }
    render(<TagBadge slug="early-access" tag={odd} />)
    expect(screen.getByText('Early Access').className).toContain('bg-gray-100')
  })

  it('omits the title when the tag has no description', () => {
    render(<TagBadge slug="early-access" tag={{ ...tag, description: undefined }} />)
    expect(screen.getByText('Early Access')).not.toHaveAttribute('title')
  })
})
