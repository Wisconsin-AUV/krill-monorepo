import { useMutation } from '@connectrpc/connect-query'
import { useState } from 'react'
import { useSearchParams } from 'react-router'
import { CODE_LENGTH, CodeInput } from '@/components/auth/CodeInput'
import { useCodeSubmit } from '@/components/auth/useCodeSubmit'
import { atLeastContinue } from '@/components/auth/motion'
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
  const [step, setStep] = useState<'request' | 'reset'>('request')
  const check = usePasswordCheck(password, [login])

  const request = useMutation(AuthService.method.requestPasswordReset)
  const [sending, setSending] = useState(false)

  async function sendCode() {
    setSending(true)
    try {
      await atLeastContinue(request.mutateAsync({ login }))
      setStep('reset')
    } catch {
      // Shown from request.error.
    } finally {
      setSending(false)
    }
  }
  const resend = useMutation(AuthService.method.requestPasswordReset, {
    onSuccess: () => flash.success('Code requested'),
  })
  // A new session cookie is set, so the auth layout moves on to the app.
  const reset = useMutation(AuthService.method.resetPassword)
  const entry = useCodeSubmit(
    (code) => reset.mutateAsync({ login, code, newPassword: password }),
    refreshSession,
  )

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
            {sending ? (
              <>
                <LoadingSpinner className="my-1 size-4" />
                <span className="sr-only">Sending code</span>
              </>
            ) : (
              'Send code'
            )}
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
          void entry.submit()
        }}
      >
        <div className="space-y-2 text-center">
          <Heading>Reset your password</Heading>
          <Text>If that account exists, we emailed it a code.</Text>
        </div>
        <FormError error={reset.error ?? resend.error} />
        <input type="text" name="username" autoComplete="username" value={login} readOnly hidden />
        <Field>
          <Label>Code</Label>
          <CodeInput value={entry.code} onChange={entry.setCode} shake={entry.shake} />
        </Field>
        <Field>
          <Label>New password</Label>
          <PasswordInput value={password} onChange={setPassword} check={check} placement="right" />
        </Field>
        <Button
          type="submit"
          color="sky"
          className="w-full"
          disabled={entry.code.length !== CODE_LENGTH || !check.ok || entry.submitting}
        >
          {entry.submitting ? (
            <>
              <LoadingSpinner className="my-1 size-4" />
              <span className="sr-only">Resetting password</span>
            </>
          ) : (
            'Reset password'
          )}
        </Button>
        <div className="flex justify-between">
          <TextButton onClick={() => resend.mutate({ login })} disabled={resend.isPending}>
            Send a new code
          </TextButton>
          <TextButton onClick={() => setStep('request')}>Change account</TextButton>
        </div>
      </form>
    </PageContentBlock>
  )
}
