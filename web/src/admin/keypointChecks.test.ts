import { create } from '@bufbuild/protobuf'
import { describe, expect, it } from 'vitest'
import { KeypointCheckSchema, KeypointCheckTypeSchema } from '@/gen/krill/v1/label_pb'
import { anyChecking, formatPixels, foundShare } from './keypointChecks'

describe('formatPixels', () => {
  it('keeps a decimal under 10 px', () => {
    expect(formatPixels(3.26)).toBe('3.3 px')
    expect(formatPixels(0)).toBe('0.0 px')
    expect(formatPixels(46.6)).toBe('47 px')
  })
})

describe('foundShare', () => {
  it('is the share of boxes the design was found in', () => {
    expect(foundShare(create(KeypointCheckSchema, { boxCount: 40, foundCount: 30 }))).toBe(0.75)
    expect(foundShare(create(KeypointCheckSchema, {}))).toBe(0)
  })
})

describe('anyChecking', () => {
  it('is true while a type has a check running', () => {
    const idle = create(KeypointCheckTypeSchema, { last: create(KeypointCheckSchema, {}) })
    const busy = create(KeypointCheckTypeSchema, { running: create(KeypointCheckSchema, {}) })
    expect(anyChecking([idle])).toBe(false)
    expect(anyChecking([idle, busy])).toBe(true)
    expect(anyChecking(undefined)).toBe(false)
  })
})
