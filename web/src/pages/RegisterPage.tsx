import { useMutation } from '@connectrpc/connect-query'
import { useState } from 'react'
import { useSearchParams } from 'react-router'
import { FormError, SlackButton } from '@/components/AuthForm'
import { Button } from '@/components/ui/Button'
import { Description, Field, FieldGroup, Label } from '@/components/ui/Fieldset'
import { Heading } from '@/components/ui/Heading'
import { Input } from '@/components/ui/Input'
import PageContentBlock from '@/components/ui/PageContentBlock'
import { Text, TextLink } from '@/components/ui/Text'
import { AuthService } from '@/gen/krill/v1/auth_pb'
import { refreshSession, useSession, withNext } from '@/lib/auth'
import { errorMessage } from '@/lib/errors'

export function RegisterPage() {
  const { data } = useSession()
  const [params] = useSearchParams()
  const [form, setForm] = useState({ name: '', username: '', email: '', password: '' })
  const [error, setError] = useState<string | null>(null)
  const register = useMutation(AuthService.method.register, {
    onSuccess: () => refreshSession(),
    onError: (err) => setError(errorMessage(err)),
  })
  const set = (key: keyof typeof form) => (e: React.ChangeEvent<HTMLInputElement>) =>
    setForm((f) => ({ ...f, [key]: e.target.value }))

  const signInLink = (
    <Text className="mt-8">
      Have an account? <TextLink to={withNext('/login', params)}>Sign in</TextLink>
    </Text>
  )

  if (!data?.signupEnabled) {
    return (
      <PageContentBlock title="Create account · Krill" width="">
        <Heading className="mt-6">Sign-up is closed</Heading>
        <Text className="mt-2">Ask an admin to create an account for you.</Text>
        {data?.slackEnabled && <SlackButton label="Continue with Slack" />}
        {signInLink}
      </PageContentBlock>
    )
  }

  return (
    <PageContentBlock title="Create account · Krill" width="">
      <Heading className="mt-6">Create an account</Heading>
      <FormError message={error} />
      <form
        className="mt-6"
        onSubmit={(e) => {
          e.preventDefault()
          setError(null)
          register.mutate(form)
        }}
      >
        <FieldGroup>
          <Field>
            <Label>Name</Label>
            <Input
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
              value={form.email}
              onChange={set('email')}
              autoComplete="email"
              required
            />
          </Field>
          <Field>
            <Label>Password</Label>
            <Description>At least 8 characters.</Description>
            <Input
              type="password"
              value={form.password}
              onChange={set('password')}
              autoComplete="new-password"
              required
              minLength={8}
            />
          </Field>
        </FieldGroup>
        <Button type="submit" color="sky" className="mt-8 w-full" disabled={register.isPending}>
          Create account
        </Button>
      </form>
      {data.slackEnabled && <SlackButton label="Continue with Slack" />}
      {signInLink}
    </PageContentBlock>
  )
}
