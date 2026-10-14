import { useRef, useState } from 'react'
import { CODE_LENGTH } from '@/components/auth/CodeInput'
import { atLeastContinue } from '@/components/auth/motion'

// Submits a code, holding the spinner a moment when it's right before moving
// on, and clearing and shaking the boxes when it's wrong.
export function useCodeSubmit(submit: (code: string) => Promise<unknown>, onDone: () => unknown) {
  const [code, setCode] = useState('')
  const [shake, setShake] = useState(0)
  const [submitting, setSubmitting] = useState(false)
  const busy = useRef(false)

  async function run(value: string) {
    if (busy.current || value.length !== CODE_LENGTH) return
    busy.current = true
    setSubmitting(true)
    try {
      try {
        await atLeastContinue(submit(value))
      } catch {
        setCode('')
        setShake((n) => n + 1)
        return
      }
      await onDone()
    } finally {
      busy.current = false
      setSubmitting(false)
    }
  }

  return {
    code,
    shake,
    submitting,
    submit: () => run(code),
    setCode,
    // Sends the code as soon as the last digit is in.
    autoSubmit: (value: string) => {
      setCode(value)
      void run(value)
    },
  }
}
