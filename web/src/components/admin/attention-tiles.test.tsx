import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { ClipboardCheck, Flag } from 'lucide-react'
import { AttentionTiles, type AttentionTile } from './attention-tiles'

function tile(overrides: Partial<AttentionTile>): AttentionTile {
  return { key: 'k', count: 1, label: 'thing', to: '/admin/x', icon: ClipboardCheck, tone: 'attention', ...overrides }
}

function renderTiles(tiles: AttentionTile[]) {
  return render(
    <MemoryRouter>
      <AttentionTiles tiles={tiles} />
    </MemoryRouter>,
  )
}

describe('AttentionTiles', () => {
  it('links each tile with a count to its page', () => {
    renderTiles([
      tile({ key: 'review', count: 3, label: 'awaiting your review', to: '/admin/review' }),
      tile({ key: 'reports', count: 1, label: 'open report', to: '/admin/reports', icon: Flag, tone: 'danger' }),
    ])
    expect(screen.getByRole('link', { name: /3\s*awaiting your review/i })).toHaveAttribute('href', '/admin/review')
    expect(screen.getByRole('link', { name: /1\s*open report/i })).toHaveAttribute('href', '/admin/reports')
  })

  it('drops tiles with nothing waiting', () => {
    renderTiles([tile({ key: 'review', count: 2, label: 'review' }), tile({ key: 'drafts', count: 0, label: 'drafts' })])
    expect(screen.getAllByRole('link')).toHaveLength(1)
  })

  it('renders nothing when no tile has a count', () => {
    const { container } = renderTiles([tile({ count: 0 })])
    expect(container).toBeEmptyDOMElement()
  })
})
