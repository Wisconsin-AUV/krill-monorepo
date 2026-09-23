import { describe, expect, it } from 'vitest'
import { initials, safeNext } from './auth'

describe('initials', () => {
  it('uses the first and last names', () => {
    expect(initials('Ada King Lovelace')).toBe('AL')
    expect(initials('  grace ')).toBe('GR')
    expect(initials('')).toBe('?')
  })
})

describe('safeNext', () => {
  it('keeps same-origin paths only', () => {
    expect(safeNext('/clips/3?frame=2')).toBe('/clips/3?frame=2')
    expect(safeNext('//evil.example')).toBe('/')
    expect(safeNext('/\\evil.example')).toBe('/')
    expect(safeNext('https://evil.example')).toBe('/')
    expect(safeNext(null)).toBe('/')
  })
})
