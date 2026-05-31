import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { ActivityTimeline } from './activity-timeline'
import type { components } from '@/lib/schema'

type ActivityEvent = components['schemas']['PublisherActivityEvent']

function ev(partial: Partial<ActivityEvent> & Pick<ActivityEvent, 'id'>): ActivityEvent {
  return {
    action: 'mcp_server.created',
    resource_type: 'mcp_server',
    resource_slug: 'weather',
    created_at: new Date().toISOString(),
    ...partial,
  }
}

describe('ActivityTimeline', () => {
  it('renders actor, a human verb, and the target for a published version', () => {
    render(
      <ActivityTimeline
        events={[
          ev({
            id: '1',
            action: 'mcp_server_version.published',
            resource_slug: 'weather',
            version: '1.4.0',
            actor_email: 'dana@x.test',
          }),
        ]}
      />,
    )
    const item = screen.getByRole('listitem')
    expect(item.textContent).toContain('dana@x.test')
    expect(item.textContent).toContain('published')
    expect(item.textContent).toContain('weather@1.4.0')
  })

  it('falls back to "Someone" when the actor email is absent', () => {
    render(<ActivityTimeline events={[ev({ id: '2' })]} />)
    expect(screen.getByRole('listitem').textContent).toContain('Someone')
  })

  it('humanizes an unknown action verb', () => {
    render(<ActivityTimeline events={[ev({ id: '3', action: 'role_grant.created', resource_slug: '' })]} />)
    expect(screen.getByRole('listitem').textContent).toContain('created')
  })
})
