import {
  CheckCircleIcon,
  EyeIcon,
  EyeSlashIcon,
  InformationCircleIcon,
} from '@heroicons/react/16/solid'
import { clsx } from 'clsx'
import { useAnimate, useReducedMotion } from 'motion/react'
import { useEffect, useRef, useState } from 'react'
import { Input } from '@/components/ui/Input'
import { MIN_CHAR_TYPES, MIN_PASSWORD_LENGTH, MIN_SCORE, type PasswordCheck } from '@/lib/password'

function Mark({ met }: { met: boolean }) {
  return met ? (
    <CheckCircleIcon className="size-4 shrink-0 text-emerald-500" />
  ) : (
    <span className="flex size-4 shrink-0 items-center justify-center">
      <span className="size-2.5 rounded-full border border-current" />
    </span>
  )
}

function Rule({ met, children }: { met: boolean; children: React.ReactNode }) {
  return (
    <li
      className={clsx(
        'flex items-center gap-2',
        met ? 'text-zinc-950 dark:text-white' : 'text-zinc-500 dark:text-zinc-400',
      )}
    >
      <Mark met={met} />
      {children}
    </li>
  )
}

function StrengthMeter({ score }: { score: number }) {
  const color = score >= MIN_SCORE ? 'bg-emerald-500' : score >= 2 ? 'bg-amber-500' : 'bg-red-500'
  return (
    <div className="flex gap-1" aria-hidden="true">
      {[1, 2, 3, 4].map((i) => (
        <span
          key={i}
          className={clsx(
            'h-1 flex-1 rounded-full',
            i <= score ? color : 'bg-zinc-950/10 dark:bg-white/10',
          )}
        />
      ))}
    </div>
  )
}

function Requirements({
  check,
  placement,
}: {
  check: PasswordCheck
  placement: 'right' | 'below'
}) {
  return (
    <div
      aria-live="polite"
      className={clsx(
        'z-20 rounded-xl bg-white p-4 text-sm/6 shadow-lg ring-1 ring-zinc-950/10 dark:bg-zinc-800 dark:ring-white/10',
        placement === 'right'
          ? 'mt-2 lg:absolute lg:top-0 lg:left-full lg:mt-0 lg:ml-4 lg:w-80'
          : 'mt-2',
      )}
    >
      <ul className="space-y-1.5">
        <Rule met={check.longEnough}>At least {MIN_PASSWORD_LENGTH} characters</Rule>
        <Rule met={check.enoughTypes}>At least {MIN_CHAR_TYPES} of these:</Rule>
        <li>
          <ul className="grid grid-cols-2 gap-x-4 gap-y-1 pl-6 text-xs/5 whitespace-nowrap *:gap-1.5">
            {check.types.map((t) => (
              <Rule key={t.label} met={t.met}>
                {t.label}
              </Rule>
            ))}
          </ul>
        </li>
        <Rule met={check.strong}>Hard to guess</Rule>
      </ul>
      {check.score !== null && (
        <div className="mt-3">
          <StrengthMeter score={check.score} />
        </div>
      )}
      {check.feedback.map((f) => (
        <p key={f} className="mt-2 flex gap-2 text-zinc-600 dark:text-zinc-300">
          <InformationCircleIcon className="mt-1 size-4 shrink-0 text-zinc-400" />
          {f}
        </p>
      ))}
    </div>
  )
}

// Lets Safari, iCloud Keychain, and 1Password generate passwords that pass
// the length and character-type rules. The attribute isn't in React's types.
const passwordRules = {
  passwordrules: `minlength: ${MIN_PASSWORD_LENGTH}; required: lower; required: upper; required: digit; required: special;`,
} as object

const SHAKE_MS = 400

export function PasswordInput({
  value,
  onChange,
  check,
  placement = 'below',
  name,
  autoComplete,
  invalid = false,
  shake = 0,
  inputRef,
}: {
  value: string
  onChange: (value: string) => void
  // Set for new passwords to show the requirements while typing.
  check?: PasswordCheck
  placement?: 'right' | 'below'
  name?: string
  autoComplete?: string
  invalid?: boolean
  // Changing this shakes the field, with the ring red until it settles.
  shake?: number
  inputRef?: React.Ref<HTMLInputElement>
}) {
  const [visible, setVisible] = useState(false)
  const [focused, setFocused] = useState(false)
  // The ring stays red until the latest shake has played out.
  const [settledShake, setSettledShake] = useState(shake)
  const flash = shake !== settledShake
  const [scope, animate] = useAnimate<HTMLDivElement>()
  const reduceMotion = useReducedMotion()
  const Icon = visible ? EyeIcon : EyeSlashIcon

  // Only a change after mount means a new failure, so a field that remounts
  // with an old count stays still.
  const lastShake = useRef(shake)
  useEffect(() => {
    if (shake === lastShake.current) return
    lastShake.current = shake
    if (!reduceMotion) {
      void animate(scope.current, { x: [0, 5, -4, 4, -2, 0] }, { duration: SHAKE_MS / 1000 })
    }
    const done = setTimeout(() => setSettledShake(shake), SHAKE_MS)
    return () => clearTimeout(done)
  }, [shake, reduceMotion, animate, scope])

  const label = visible ? 'Hide password' : 'Show password'

  return (
    // Marked as the control so Field spaces it from its label like any input.
    <div data-slot="control" className="relative">
      <div ref={scope} className="relative [&_input]:pr-11">
        <Input
          ref={inputRef}
          type={visible ? 'text' : 'password'}
          name={name ?? (check ? 'new-password' : 'password')}
          {...(check && passwordRules)}
          value={value}
          onChange={(e) => onChange(e.target.value)}
          onFocus={() => setFocused(true)}
          onBlur={() => setFocused(false)}
          autoComplete={autoComplete ?? (check ? 'new-password' : 'current-password')}
          required
          invalid={invalid}
          alert={flash}
        />
        <button
          type="button"
          onClick={() => setVisible((v) => !v)}
          aria-label={label}
          title={label}
          className="absolute top-1/2 right-1.5 z-10 -translate-y-1/2 rounded-md p-1.5 text-zinc-500 hover:bg-zinc-950/5 hover:text-zinc-700 dark:text-zinc-400 dark:hover:bg-white/10 dark:hover:text-zinc-200"
        >
          <Icon className="size-4" />
        </button>
      </div>
      {/* Waits for typing so it doesn't cover a password manager's menu, which opens on focus. */}
      {check && focused && value && <Requirements check={check} placement={placement} />}
    </div>
  )
}
