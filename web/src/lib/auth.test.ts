import { describe, expect, it } from 'vitest'
import { safeNext } from './auth'

describe('safeNext', () => {
  it('keeps same-origin paths only', () => {
    expect(safeNext('/clips/3?frame=2')).toBe('/clips/3?frame=2')
    expect(safeNext('//evil.example')).toBe('/')
    expect(safeNext('/\\evil.example')).toBe('/')
    expect(safeNext('https://evil.example')).toBe('/')
    expect(safeNext(null)).toBe('/')
  })
})
