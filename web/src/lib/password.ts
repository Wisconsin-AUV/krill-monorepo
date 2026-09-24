import type { ZxcvbnResult } from '@zxcvbn-ts/core'
import { useEffect, useMemo, useState } from 'react'

export const MIN_PASSWORD_LENGTH = 8
export const MIN_CHAR_TYPES = 3
// zxcvbn's top score, at least 10^10 estimated guesses. Score 3 still lets
// through passwords like "Aaaaaaaa1!".
export const MIN_SCORE = 4

export const charTypes = [
  { label: 'Uppercase letter', test: /\p{Lu}/u },
  { label: 'Lowercase letter', test: /\p{Ll}/u },
  { label: 'Number', test: /\p{N}/u },
  { label: 'Symbol', test: /[^\p{L}\p{N}]/u },
] as const

type Checker = (password: string, userInputs: string[]) => ZxcvbnResult

let checker: Promise<Checker> | null = null

// The dictionaries are large, so they load only on pages that set passwords.
export function loadChecker(): Promise<Checker> {
  checker ??= Promise.all([
    import('@zxcvbn-ts/core'),
    import('@zxcvbn-ts/language-common'),
    import('@zxcvbn-ts/language-en'),
  ]).then(([core, common, en]) => {
    const zxcvbn = new core.ZxcvbnFactory({
      translations: en.translations,
      graphs: common.adjacencyGraphs,
      dictionary: { ...common.dictionary, ...en.dictionary },
    })
    return (password, userInputs) => zxcvbn.check(password, userInputs)
  })
  return checker
}

export interface PasswordCheck {
  longEnough: boolean
  types: { label: string; met: boolean }[]
  enoughTypes: boolean
  // Null until the strength checker loads.
  score: number | null
  strong: boolean
  feedback: string[]
  ok: boolean
}

export function checkPassword(
  password: string,
  strength?: (password: string) => ZxcvbnResult,
): PasswordCheck {
  const longEnough = [...password].length >= MIN_PASSWORD_LENGTH
  const types = charTypes.map((t) => ({ label: t.label, met: t.test.test(password) }))
  const enoughTypes = types.filter((t) => t.met).length >= MIN_CHAR_TYPES
  const result = password && strength ? strength(password) : null
  const score = result?.score ?? null
  const strong = score !== null && score >= MIN_SCORE
  const feedback = result
    ? [result.feedback.warning, ...result.feedback.suggestions].filter((f): f is string => !!f)
    : []
  return {
    longEnough,
    types,
    enoughTypes,
    score,
    strong,
    feedback,
    ok: longEnough && enoughTypes && strong,
  }
}

// userInputs are words the password should not be built from, such as the
// user's name and email.
export function usePasswordCheck(password: string, userInputs: string[] = []): PasswordCheck {
  const [check, setCheck] = useState<Checker | null>(null)
  useEffect(() => {
    let active = true
    void loadChecker().then((c) => {
      if (active) setCheck(() => c)
    })
    return () => {
      active = false
    }
  }, [])
  const inputs = userInputs.join('\n')
  return useMemo(
    () =>
      checkPassword(
        password,
        check ? (p) => check(p, inputs.split('\n').filter(Boolean)) : undefined,
      ),
    [password, check, inputs],
  )
}
