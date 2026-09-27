import { describe, expect, it } from 'vitest'
import { exportClasses, withAlpha } from './labels'

describe('exportClasses', () => {
  it('matches the API flattening order', () => {
    expect(
      exportClasses('torpedo_hole', [
        { name: 'size', options: ['big', 'small'] },
        { name: 'role', options: ['red', 'blue'] },
      ]),
    ).toEqual([
      'torpedo_hole-big-red',
      'torpedo_hole-big-blue',
      'torpedo_hole-small-red',
      'torpedo_hole-small-blue',
    ])
    expect(exportClasses('gate', [])).toEqual(['gate'])
  })
})

describe('withAlpha', () => {
  it('appends an alpha byte', () => {
    expect(withAlpha('#0ea5e9', 0.5)).toBe('#0ea5e980')
  })
})
