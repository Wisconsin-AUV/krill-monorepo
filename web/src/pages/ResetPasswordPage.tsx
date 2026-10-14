import { ConnectError } from '@connectrpc/connect'
import { useMutation } from '@connectrpc/connect-query'
import { AnimatePresence, motion } from 'motion/react'
import { useState } from 'react'
import { useSearchParams } from 'react-router'
import { CODE_LENGTH, CodeInput } from '@/components/auth/CodeInput'
import { useCodeSubmit } from '@/components/auth/useCodeSubmit'
import { atLeastContinue, swapStep } from '@/components/auth/motion'
import { PasswordInput } from '@/components/PasswordInput'
import { Button } from '@/components/ui/Button'
import { FormError } from '@/components/ui/Callout'
import { Field, Label } from '@/components/ui/Fieldset'
import { Heading } from '@/components/ui/Heading'
import { Input } from '@/components/ui/Input'
import LoadingSpinner from '@/components/ui/LoadingSpinner'
import PageContentBlock from '@/components/ui/PageContentBlock'
import { Strong, Text, TextButton, TextLink } from '@/components/ui/Text'
import { AuthService } from '@/gen/krill/v1/auth_pb'
import { refreshSession, useSession } from '@/lib/auth'
import { flash } from '@/lib/flash'
import { usePasswordCheck } from '@/lib/password'

export function ResetPasswordPage() {
  const { data } = useSession()
  const [params] = useSearchParams()
  const [login, setLogin] = useState(params.get('login') ?? '')
  const [password, setPassword] = useState('')
  const [confirm, setConfirm] = useState('')
  const [step, setStep] = useState<'request' | 'code' | 'password'>('request')
  const check = usePasswordCheck(password, [login])
  const mismatch = confirm !== '' && confirm !== password

  const request = useMutation(AuthService.method.requestPasswordReset)
  const [sending, setSending] = useState(false)
  async function sendCode() {
    setSending(true)
    try {
      await atLeastContinue(request.mutateAsync({ login }))
      setStep('code')
    } catch {
      // Shown from request.error.
    } finally {
      setSending(false)
    }
  }
  const resend = useMutation(AuthService.method.requestPasswordReset, {
    onSuccess: () => flash.success('Code requested'),
  })

  const reset = useMutation(AuthService.method.resetPassword)
  const checkCode = useMutation(AuthService.method.checkResetCode)
  const entry = useCodeSubmit(
    (code) => checkCode.mutateAsync({ login, code }),
    () => {
      reset.reset()
      setStep('password')
    },
  )

  const [resetting, setResetting] = useState(false)
  async function resetPassword() {
    setResetting(true)
    try {
      await atLeastContinue(reset.mutateAsync({ login, code: entry.code, newPassword: password }))
      // A new session cookie is set, so the auth layout moves on to the app.
      await refreshSession()
    } catch (err) {
      // The code ran out while choosing a password, so ask for it again.
      if (err instanceof ConnectError && err.rawMessage === BAD_CODE) {
        entry.setCode('')
        setStep('code')
      }
    } finally {
      setResetting(false)
    }
  }

  const signInLink = (
    <Text className="text-center">
      <TextLink to="/login">
        <Strong>Back to sign in</Strong>
      </TextLink>
    </Text>
  )

  if (!data?.emailCodesEnabled) {
    return (
      <PageContentBlock title="Reset password · Krill" width="w-full max-w-sm">
        <div className="grid grid-cols-1 gap-8">
          <Heading className="text-center">Password reset is off</Heading>
          <Text className="text-center">Ask an admin to set a new password for you.</Text>
          {signInLink}
        </div>
      </PageContentBlock>
    )
  }

  if (step === 'request') {
    return (
      <PageContentBlock title="Reset password · Krill" width="w-full max-w-sm">
        <form
          className="grid grid-cols-1 gap-8"
          onSubmit={(e) => {
            e.preventDefault()
            void sendCode()
          }}
        >
          <Heading className="text-center">Reset your password</Heading>
          <FormError error={request.error} />
          <Field>
            <Label>Username or email</Label>
            <Input
              name="username"
              value={login}
              onChange={(e) => setLogin(e.target.value)}
              autoComplete="username"
              required
            />
          </Field>
          <Button type="submit" color="sky" className="w-full" disabled={sending}>
            <Spinning when={sending} label="Sending code">
              Send code
            </Spinning>
          </Button>
          {signInLink}
        </form>
      </PageContentBlock>
    )
  }

  return (
    <PageContentBlock title="Reset password · Krill" width="w-full max-w-sm">
      <form
        className="grid grid-cols-1 gap-8"
        onSubmit={(e) => {
          e.preventDefault()
          if (step === 'code') void entry.submit()
          else void resetPassword()
        }}
      >
        <div className="space-y-2 text-center">
          <Heading>Reset your password</Heading>
          <Text>
            {step === 'code'
              ? 'If that account exists, we emailed it a code.'
              : 'Choose a new password.'}
          </Text>
        </div>
        <FormError error={reset.error ?? checkCode.error ?? resend.error} />
        <input type="text" name="username" autoComplete="username" value={login} readOnly hidden />
        <AnimatePresence initial={false}>
          {step === 'code' ? (
            <motion.div key="code" {...swapStep} className="grid grid-cols-1 gap-8">
              <Field>
                <Label>Code</Label>
                <CodeInput value={entry.code} onChange={entry.autoSubmit} shake={entry.shake} />
              </Field>
              <Button
                type="submit"
                color="sky"
                className="w-full"
                disabled={entry.code.length !== CODE_LENGTH || entry.submitting}
              >
                <Spinning when={entry.submitting} label="Checking code">
                  Continue
                </Spinning>
              </Button>
              <div className="flex justify-between">
                <TextButton onClick={() => resend.mutate({ login })} disabled={resend.isPending}>
                  Send a new code
                </TextButton>
                <TextButton
                  onClick={() => {
                    entry.setCode('')
                    setStep('request')
                  }}
                >
                  Change account
                </TextButton>
              </div>
            </motion.div>
          ) : (
            <motion.div key="password" {...swapStep} className="grid grid-cols-1 gap-8">
              <Field>
                <Label>New password</Label>
                <PasswordInput
                  value={password}
                  onChange={setPassword}
                  check={check}
                  placement="right"
                />
              </Field>
              <Field error={mismatch ? 'Passwords do not match' : undefined}>
                <Label>Confirm new password</Label>
                <PasswordInput
                  value={confirm}
                  onChange={setConfirm}
                  name="confirm-password"
                  autoComplete="new-password"
                  invalid={mismatch}
                />
              </Field>
              <Button
                type="submit"
                color="sky"
                className="w-full"
                disabled={mismatch || !confirm || !check.ok || resetting}
              >
                <Spinning when={resetting} label="Resetting password">
                  Reset password
                </Spinning>
              </Button>
            </motion.div>
          )}
        </AnimatePresence>
      </form>
    </PageContentBlock>
  )
}

const BAD_CODE = 'that code is incorrect or has expired'

function Spinning({
  when,
  label,
  children,
}: {
  when: boolean
  label: string
  children: React.ReactNode
}) {
  if (!when) return children
  return (
    <>
      <LoadingSpinner className="my-1 size-4" />
      <span className="sr-only">{label}</span>
    </>
  )
}
