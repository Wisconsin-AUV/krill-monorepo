import { describe, expect, it } from 'vitest'
import { formatBytes, formatDuration, formatFrameTime, formatRelative, plural } from './format'

describe('formatDuration', () => {
  it.each([
    [0, '0:00'],
    [9_400, '0:09'],
    [75_000, '1:15'],
    [3_725_000n, '1:02:05'],
  ])('%s ms -> %s', (ms, want) => {
    expect(formatDuration(ms)).toBe(want)
  })
})

describe('formatBytes', () => {
  it.each([
    [512, '512 B'],
    [1536, '1.5 KB'],
    [250 * 1024 * 1024, '250 MB'],
  ])('%s -> %s', (bytes, want) => {
    expect(formatBytes(bytes)).toBe(want)
  })
})

describe('formatRelative', () => {
  it('rounds to the largest unit', () => {
    const now = new Date('2026-01-10T12:00:00Z')
    expect(formatRelative(new Date('2026-01-10T11:59:30Z'), now)).toBe('just now')
    expect(formatRelative(new Date('2026-01-10T09:00:00Z'), now)).toBe('3 hours ago')
    expect(formatRelative(new Date('2026-01-09T12:00:00Z'), now)).toBe('yesterday')
  })
})

describe('formatFrameTime', () => {
  it('shows tenths without rounding the seconds up', () => {
    expect(formatFrameTime(0)).toBe('0:00.0')
    expect(formatFrameTime(12_960)).toBe('0:12.9')
  })
})

describe('plural', () => {
  it('picks the word form from the count', () => {
    expect(plural(1, 'image')).toBe('1 image')
    expect(plural(2, 'image')).toBe('2 images')
    expect(plural(1200, 'box', 'boxes')).toBe('1,200 boxes')
  })
})
