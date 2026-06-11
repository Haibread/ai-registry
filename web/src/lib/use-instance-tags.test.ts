/**
 * use-instance-tags.test.ts
 *
 * Tests for the pure helpers around the instance-tag vocabulary: the
 * slug → tag index used to resolve frozen version slugs, the active-only
 * filter that feeds the publish-form picker, and the FormData collector
 * that turns ticked picker checkboxes back into slugs.
 */

import { describe, it, expect } from 'vitest'
import {
  indexInstanceTags,
  activeInstanceTags,
  collectInstanceTags,
  INSTANCE_TAG_FIELD_PREFIX,
  type InstanceTag,
} from './use-instance-tags'

function makeTag(overrides: Partial<InstanceTag> & Pick<InstanceTag, 'slug' | 'name'>): InstanceTag {
  return {
    active: true,
    created_at: '2026-06-01T10:00:00Z',
    updated_at: '2026-06-01T10:00:00Z',
    ...overrides,
  }
}

const earlyAccess = makeTag({ slug: 'early-access', name: 'Early Access', color: 'blue' })
const legacy = makeTag({ slug: 'legacy', name: 'Legacy', active: false })

describe('indexInstanceTags', () => {
  it('builds a slug → tag lookup including deactivated tags', () => {
    const index = indexInstanceTags([earlyAccess, legacy])
    expect(index.get('early-access')).toBe(earlyAccess)
    // Deactivated definitions stay resolvable — frozen versions carry them.
    expect(index.get('legacy')).toBe(legacy)
    expect(index.get('unknown')).toBeUndefined()
  })

  it('returns an empty map for undefined (vocabulary not loaded yet)', () => {
    expect(indexInstanceTags(undefined).size).toBe(0)
  })
})

describe('activeInstanceTags', () => {
  it('keeps only the tags publishers may tick on a new version', () => {
    expect(activeInstanceTags([earlyAccess, legacy])).toEqual([earlyAccess])
  })

  it('returns an empty list for undefined (vocabulary not loaded yet)', () => {
    expect(activeInstanceTags(undefined)).toEqual([])
  })
})

describe('collectInstanceTags', () => {
  it('extracts the slugs of ticked picker checkboxes from the form', () => {
    const fd = new FormData()
    fd.append(`${INSTANCE_TAG_FIELD_PREFIX}early-access`, 'on')
    fd.append(`${INSTANCE_TAG_FIELD_PREFIX}beta`, 'on')
    expect(collectInstanceTags(fd)).toEqual(['early-access', 'beta'])
  })

  it('ignores unrelated form fields', () => {
    const fd = new FormData()
    fd.append('name', 'My Server')
    fd.append('description', 'instance_tag_decoy in a value, not a key')
    fd.append(`${INSTANCE_TAG_FIELD_PREFIX}legacy`, 'on')
    expect(collectInstanceTags(fd)).toEqual(['legacy'])
  })

  it('returns an empty list when nothing is ticked', () => {
    expect(collectInstanceTags(new FormData())).toEqual([])
  })
})
