import * as Headless from '@headlessui/react'
import { clsx } from 'clsx'
import { useState } from 'react'
import { useShake } from '@/components/useShake'

export const CODE_LENGTH = 6

// One real input under the boxes, so typing, backspace, paste, and one-time
// code autofill all behave natively. The caret is held at the end, so digits
// fill in order.
export function CodeInput({
  value,
  onChange,
  shake = 0,
}: {
  value: string
  onChange: (value: string) => void
  // Changing this shakes the boxes, red until it settles.
  shake?: number
}) {
  const [focused, setFocused] = useState(false)
  const { scope, shaking } = useShake(shake)
  const active = Math.min(value.length, CODE_LENGTH - 1)

  function keepCaretAtEnd(input: HTMLInputElement) {
    const end = input.value.length
    if (input.selectionStart !== end || input.selectionEnd !== end) {
      input.setSelectionRange(end, end)
    }
  }

  return (
    <div ref={scope} data-slot="control" className="relative grid w-full grid-cols-6 gap-2">
      <Headless.Input
        name="code"
        value={value}
        onChange={(e) => onChange(e.target.value.replace(/\D/g, '').slice(0, CODE_LENGTH))}
        onFocus={(e) => {
          setFocused(true)
          keepCaretAtEnd(e.currentTarget)
        }}
        onBlur={() => setFocused(false)}
        onSelect={(e) => keepCaretAtEnd(e.currentTarget)}
        inputMode="numeric"
        autoComplete="one-time-code"
        maxLength={CODE_LENGTH}
        spellCheck={false}
        required
        className="absolute inset-0 z-10 size-full cursor-text opacity-0"
      />
      {Array.from({ length: CODE_LENGTH }, (_, i) => {
        const current = focused && i === active
        return (
          <div
            key={i}
            aria-hidden
            className={clsx(
              'flex aspect-square items-center justify-center rounded-lg border bg-white text-xl font-semibold text-zinc-950 tabular-nums shadow-sm dark:bg-white/5 dark:text-white',
              shaking
                ? 'border-red-500 ring-1 ring-red-500'
                : current
                  ? 'border-blue-500 ring-1 ring-blue-500'
                  : 'border-zinc-950/10 dark:border-white/10',
            )}
          >
            {value[i] ??
              (current && (
                <span className="animate-caret-blink h-6 w-px bg-zinc-950 dark:bg-white" />
              ))}
          </div>
        )
      })}
    </div>
  )
}
