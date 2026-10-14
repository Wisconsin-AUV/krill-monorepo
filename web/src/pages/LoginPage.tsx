import { useMutation } from '@connectrpc/connect-query'
import { ChevronLeftIcon, EnvelopeIcon } from '@heroicons/react/24/outline'
import { AnimatePresence, motion } from 'motion/react'
import { useEffect, useRef, useState } from 'react'
import { useSearchParams } from 'react-router'
import { CODE_LENGTH, CodeInput } from '@/components/auth/CodeInput'
import { useCodeSubmit } from '@/components/auth/useCodeSubmit'
import { LastUsedBadge } from '@/components/auth/LastUsedBadge'
import { OrDivider } from '@/components/auth/OrDivider'
import {
  atLeastContinue,
  CONTINUE_MS,
  fadeWithPassword,
  revealPassword,
} from '@/components/auth/motion'
import { SlackButton } from '@/components/auth/SlackButton'
import { PasswordInput } from '@/components/PasswordInput'
import { Button } from '@/components/ui/Button'
import { FormError } from '@/components/ui/Callout'
import { Field, Label, LabelRow } from '@/components/ui/Fieldset'
import { Heading } from '@/components/ui/Heading'
import { Input } from '@/components/ui/Input'
import LoadingSpinner from '@/components/ui/LoadingSpinner'
import PageContentBlock from '@/components/ui/PageContentBlock'
import { Strong, Text, TextButton, TextLink } from '@/components/ui/Text'
import { AuthService } from '@/gen/krill/v1/auth_pb'
import { lastLoginMethod, refreshSession, useSession, withNext } from '@/lib/auth'
import { errorMessage } from '@/lib/errors'
import { flash } from '@/lib/flash'

export function LoginPage() {
  const { data } = useSession()
  const [params, setParams] = useSearchParams()
  const [login, setLogin] = useState('')
  const [password, setPassword] = useState('')

  const [step, setStep] = useState<'login' | 'password' | 'code'>('login')
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

  const requestCode = useMutation(AuthService.method.requestSignInCode)
  const [sendingCode, setSendingCode] = useState(false)

  async function emailCode() {
    setSendingCode(true)
    try {
      await atLeastContinue(requestCode.mutateAsync({ login }))
      setError(null)
      setStep('code')
    } catch (err) {
      setError(errorMessage(err))
    } finally {
      setSendingCode(false)
    }
  }

  useEffect(() => {
    if (!params.has('error')) return
    setParams(
      (p) => {
        p.delete('error')
        return p
      },
      { replace: true },
    )
  }, [params, setParams])

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
  if (step === 'code') return <CodeSignIn login={login} onBack={() => setStep('password')} />

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
        <FormError error={error} />
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
                <LabelRow
                  action={
                    data?.emailCodesEnabled && (
                      <TextLink
                        to={`/reset-password?login=${encodeURIComponent(login)}`}
                        className="text-sm/6"
                      >
                        Forgot password?
                      </TextLink>
                    )
                  }
                >
                  Password
                </LabelRow>
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
                <LoadingSpinner className="my-1 size-4" />
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
              {data?.emailCodesEnabled && (
                <>
                  <OrDivider />
                  <Button
                    type="button"
                    outline
                    className="w-full"
                    onClick={emailCode}
                    disabled={sendingCode}
                  >
                    {sendingCode ? (
                      <>
                        <LoadingSpinner className="my-1 size-4" />
                        <span className="sr-only">Sending code</span>
                      </>
                    ) : (
                      <>
                        <EnvelopeIcon data-slot="icon" />
                        Email sign-in code
                      </>
                    )}
                  </Button>
                </>
              )}
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

function CodeSignIn({ login, onBack }: { login: string; onBack: () => void }) {
  const signIn = useMutation(AuthService.method.signInWithCode)
  const entry = useCodeSubmit((code) => signIn.mutateAsync({ login, code }), refreshSession)
  const resend = useMutation(AuthService.method.requestSignInCode, {
    onSuccess: () => flash.success('Code requested'),
  })

  return (
    <PageContentBlock title="Sign in · Krill" width="w-full max-w-sm">
      <form
        className="grid grid-cols-1 gap-8"
        onSubmit={(e) => {
          e.preventDefault()
          void entry.submit()
        }}
      >
        <div className="space-y-2 text-center">
          <Heading>Check your email</Heading>
          <Text>If that account exists, we emailed it a sign-in code.</Text>
        </div>
        <FormError error={signIn.error ?? resend.error} />
        <Field>
          <Label>Code</Label>
          <CodeInput shake={entry.shake} value={entry.code} onChange={entry.autoSubmit} />
        </Field>
        <Button
          type="submit"
          color="sky"
          className="w-full"
          disabled={entry.code.length !== CODE_LENGTH || entry.submitting}
        >
          {entry.submitting ? (
            <>
              <LoadingSpinner className="my-1 size-4" />
              <span className="sr-only">Signing in</span>
            </>
          ) : (
            'Sign in'
          )}
        </Button>
        <div className="flex justify-between">
          <TextButton onClick={() => resend.mutate({ login })} disabled={resend.isPending}>
            Send a new code
          </TextButton>
          <TextButton onClick={onBack}>Use password</TextButton>
        </div>
      </form>
    </PageContentBlock>
  )
}
