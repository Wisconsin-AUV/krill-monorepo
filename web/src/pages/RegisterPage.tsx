import { useMutation } from '@connectrpc/connect-query'
import { useState } from 'react'
import { useSearchParams } from 'react-router'
import { FormError } from '@/components/auth/FormError'
import { SlackButton } from '@/components/auth/SlackButton'
import { PasswordInput } from '@/components/PasswordInput'
import { Button } from '@/components/ui/Button'
import { Field, Label } from '@/components/ui/Fieldset'
import { Heading } from '@/components/ui/Heading'
import { Input } from '@/components/ui/Input'
import PageContentBlock from '@/components/ui/PageContentBlock'
import { Strong, Text, TextLink } from '@/components/ui/Text'
import { AuthService } from '@/gen/krill/v1/auth_pb'
import { refreshSession, useSession, withNext } from '@/lib/auth'
import { errorMessage } from '@/lib/errors'
import { usePasswordCheck } from '@/lib/password'

export function RegisterPage() {
  const { data } = useSession()
  const [params] = useSearchParams()
  const [form, setForm] = useState({ name: '', username: '', email: '', password: '' })
  const [error, setError] = useState<string | null>(null)
  const check = usePasswordCheck(form.password, [form.name, form.username, form.email])
  const register = useMutation(AuthService.method.register, {
    onSuccess: () => refreshSession(),
    onError: (err) => setError(errorMessage(err)),
  })
  const set = (key: keyof typeof form) => (e: React.ChangeEvent<HTMLInputElement>) =>
    setForm((f) => ({ ...f, [key]: e.target.value }))

  const signInLink = (
    <Text className="text-center">
      Already have an account?{' '}
      <TextLink to={withNext('/login', params)}>
        <Strong>Sign in</Strong>
      </TextLink>
    </Text>
  )

  if (!data?.signupEnabled) {
    return (
      <PageContentBlock title="Sign up · Krill" width="w-full max-w-sm">
        <div className="grid grid-cols-1 gap-8">
          <Heading className="text-center">Sign-up is closed</Heading>
          <Text className="text-center">Ask an admin to create an account for you.</Text>
          {data?.slackEnabled && <SlackButton label="Continue with Slack" />}
          {signInLink}
        </div>
      </PageContentBlock>
    )
  }

  return (
    <PageContentBlock title="Sign up · Krill" width="w-full max-w-sm">
      <form
        className="grid grid-cols-1 gap-8"
        onSubmit={(e) => {
          e.preventDefault()
          setError(null)
          register.mutate(form)
        }}
      >
        <Heading className="text-center">Create your account</Heading>
        <FormError message={error} />
        <Field>
          <Label>Name</Label>
          <Input
            name="name"
            value={form.name}
            onChange={set('name')}
            autoComplete="name"
            required
            maxLength={100}
          />
        </Field>
        <Field>
          <Label>Username</Label>
          <Input
            name="username"
            value={form.username}
            onChange={set('username')}
            autoComplete="username"
            required
            minLength={2}
            maxLength={32}
            pattern="[A-Za-z0-9][A-Za-z0-9_.\-]*"
          />
        </Field>
        <Field>
          <Label>Email</Label>
          <Input
            type="email"
            name="email"
            value={form.email}
            onChange={set('email')}
            autoComplete="email"
            required
          />
        </Field>
        <Field>
          <Label>Password</Label>
          <PasswordInput
            value={form.password}
            onChange={(password) => setForm((f) => ({ ...f, password }))}
            check={check}
            placement="right"
          />
        </Field>
        <Button
          type="submit"
          color="sky"
          className="w-full"
          disabled={register.isPending || !check.ok}
        >
          Create account
        </Button>
        {data.slackEnabled && <SlackButton label="Continue with Slack" />}
        {signInLink}
      </form>
    </PageContentBlock>
  )
}
