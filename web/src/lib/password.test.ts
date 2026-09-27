import { beforeAll, describe, expect, it } from 'vitest'
import { checkPassword, loadChecker } from './password'

let strength: (p: string) => ReturnType<Awaited<ReturnType<typeof loadChecker>>>

beforeAll(async () => {
  const check = await loadChecker()
  strength = (p) => check(p, ['ada lovelace', 'ada', 'ada@wisc.edu'])
})

describe('checkPassword', () => {
  it('requires length and three character types', () => {
    expect(checkPassword('Ab1!').longEnough).toBe(false)
    expect(checkPassword('lowercaseonly').enoughTypes).toBe(false)
    expect(checkPassword('lower1234').enoughTypes).toBe(false)
    expect(checkPassword('Lower1234').enoughTypes).toBe(true)
  })

  it.each([
    // Each passes the length and character-type rules; only the strength check stops it.
    'Abcabcabc1!',
    'Aaaaaaaa1!',
    'Ada2026!!',
  ])('rejects guessable %s', (password) => {
    expect(checkPassword(password, strength).ok).toBe(false)
  })

  it('accepts a strong password', () => {
    const check = checkPassword('Tide-pool gate 47 sonar!', strength)
    expect(check.ok).toBe(true)
    expect(check.feedback).toEqual([])
  })

  it('is not ok until the strength checker loads', () => {
    expect(checkPassword('Tide-pool gate 47 sonar!').ok).toBe(false)
  })
})
