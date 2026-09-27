import { useMutation } from '@connectrpc/connect-query'
import { ChevronLeftIcon, EnvelopeIcon } from '@heroicons/react/24/outline'
import { AnimatePresence, motion } from 'motion/react'
import { useEffect, useRef, useState } from 'react'
import { useSearchParams } from 'react-router'
import { FormError } from '@/components/auth/FormError'
import { LastUsedBadge } from '@/components/auth/LastUsedBadge'
import { OrDivider } from '@/components/auth/OrDivider'
import { fadeWithPassword, revealPassword } from '@/components/auth/motion'
import { SlackButton } from '@/components/auth/SlackButton'
import { PasswordInput } from '@/components/PasswordInput'
import { Button } from '@/components/ui/Button'
import { Field, LabelRow } from '@/components/ui/Fieldset'
import { Heading } from '@/components/ui/Heading'
import { Input } from '@/components/ui/Input'
import LoadingSpinner from '@/components/ui/LoadingSpinner'
import PageContentBlock from '@/components/ui/PageContentBlock'
import { Strong, Text, TextButton, TextLink } from '@/components/ui/Text'
import { AuthService } from '@/gen/krill/v1/auth_pb'
import { lastLoginMethod, refreshSession, useSession, withNext } from '@/lib/auth'
import { errorMessage } from '@/lib/errors'

const CONTINUE_MS = 350

export function LoginPage() {
  const { data } = useSession()
  const [params] = useSearchParams()
  const [login, setLogin] = useState('')
  const [password, setPassword] = useState('')

  const [step, setStep] = useState<'login' | 'password'>('login')
  const [continuing, setContinuing] = useState(false)
  const [error, setError] = useState<string | null>(params.get('error'))
  const [wrong, setWrong] = useState(false)
  const [shake, setShake] = useState(0)
  const [lastUsed] = useState(lastLoginMethod)
  const loginRef = useRef<HTMLInputElement>(null)
  const passwordRef = useRef<HTMLInputElement>(null)

  const signIn = useMutation(AuthService.method.login, {
    onSuccess: () => refreshSession(),
    onError: (err) => {
      setError(errorMessage(err))
      setWrong(true)
      setShake((n) => n + 1)
      passwordRef.current?.select()
    },
  })

  useEffect(() => {
    if (!continuing) return
    const t = setTimeout(() => {
      setContinuing(false)
      setStep('password')
    }, CONTINUE_MS)
    return () => clearTimeout(t)
  }, [continuing])

  useEffect(() => {
    if (step === 'password') passwordRef.current?.focus()
    else loginRef.current?.focus()
  }, [step])

  function back() {
    setStep('login')
    setPassword('')
    setWrong(false)
    setError(null)
  }

  const onPasswordStep = step === 'password'

  return (
    <PageContentBlock title="Sign in · Krill" width="w-full max-w-sm">
      <form
        className="relative grid grid-cols-1 gap-8"
        onSubmit={(e) => {
          e.preventDefault()
          if (!onPasswordStep) {
            setError(null)
            setContinuing(true)
            return
          }
          signIn.mutate({ login, password })
        }}
      >
        <Heading className="text-center">Sign in to Krill</Heading>
        <FormError message={error} />
        <Field>
          <LabelRow action={onPasswordStep && <TextButton onClick={back}>Change</TextButton>}>
            Username or email
          </LabelRow>
          <Input
            ref={loginRef}
            name="username"
            value={login}
            onChange={(e) => setLogin(e.target.value)}
            autoComplete="username"
            required
            readOnly={onPasswordStep}
            tabIndex={onPasswordStep ? -1 : undefined}
          />
        </Field>
        <AnimatePresence initial={false}>
          {onPasswordStep && (
            <motion.div key="password" {...revealPassword}>
              <Field>
                <LabelRow>Password</LabelRow>
                <PasswordInput
                  inputRef={passwordRef}
                  value={password}
                  onChange={(v) => {
                    setPassword(v)
                    setWrong(false)
                  }}
                  invalid={wrong}
                  shake={shake}
                />
              </Field>
            </motion.div>
          )}
        </AnimatePresence>
        <div className="relative">
          <Button
            type="submit"
            color="sky"
            className="w-full"
            disabled={signIn.isPending || continuing}
          >
            {signIn.isPending || continuing ? (
              <>
                <LoadingSpinner className="size-4" />
                <span className="sr-only">Signing in</span>
              </>
            ) : onPasswordStep ? (
              'Sign in'
            ) : (
              'Continue'
            )}
          </Button>
          {lastUsed === 'password' && data?.slackEnabled && <LastUsedBadge />}
        </div>
        {data?.slackEnabled && !onPasswordStep && (
          <SlackButton label="Continue with Slack" lastUsed={lastUsed === 'slack'} />
        )}
        <AnimatePresence initial={false}>
          {onPasswordStep && (
            <motion.div key="other-methods" {...fadeWithPassword} className="grid gap-8">
              <OrDivider />
              <Button type="button" outline className="w-full">
                <EnvelopeIcon data-slot="icon" />
                Email sign-in code
              </Button>
              <TextButton onClick={back} className="mt-8 flex items-center justify-center gap-1">
                <ChevronLeftIcon className="size-4" />
                Change method
              </TextButton>
            </motion.div>
          )}
        </AnimatePresence>
        {data?.signupEnabled && !onPasswordStep && (
          <Text className="text-center">
            Don’t have an account?{' '}
            <TextLink to={withNext('/register', params)}>
              <Strong>Sign up</Strong>
            </TextLink>
          </Text>
        )}
      </form>
    </PageContentBlock>
  )
}
