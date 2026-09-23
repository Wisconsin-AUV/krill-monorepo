import { useMutation } from '@connectrpc/connect-query'
import { useState } from 'react'
import { useSearchParams } from 'react-router'
import { FormError, SlackButton } from '@/components/AuthForm'
import { Button } from '@/components/ui/Button'
import { Field, FieldGroup, Label } from '@/components/ui/Fieldset'
import { Heading } from '@/components/ui/Heading'
import { Input } from '@/components/ui/Input'
import PageContentBlock from '@/components/ui/PageContentBlock'
import { Text, TextLink } from '@/components/ui/Text'
import { AuthService } from '@/gen/krill/v1/auth_pb'
import { refreshSession, useSession, withNext } from '@/lib/auth'
import { errorMessage } from '@/lib/errors'

export function LoginPage() {
  const { data } = useSession()
  const [params] = useSearchParams()
  const [login, setLogin] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState<string | null>(params.get('error'))
  const signIn = useMutation(AuthService.method.login, {
    onSuccess: () => refreshSession(),
    onError: (err) => setError(errorMessage(err)),
  })

  return (
    <PageContentBlock title="Sign in · Krill" width="">
      <Heading className="mt-6">Sign in</Heading>
      <FormError message={error} />
      <form
        className="mt-6"
        onSubmit={(e) => {
          e.preventDefault()
          setError(null)
          signIn.mutate({ login, password })
        }}
      >
        <FieldGroup>
          <Field>
            <Label>Username or email</Label>
            <Input
              value={login}
              onChange={(e) => setLogin(e.target.value)}
              autoComplete="username"
              required
            />
          </Field>
          <Field>
            <Label>Password</Label>
            <Input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              autoComplete="current-password"
              required
            />
          </Field>
        </FieldGroup>
        <Button type="submit" color="sky" className="mt-8 w-full" disabled={signIn.isPending}>
          Sign in
        </Button>
      </form>
      {data?.slackEnabled && <SlackButton label="Sign in with Slack" />}
      {data?.signupEnabled && (
        <Text className="mt-8">
          No account? <TextLink to={withNext('/register', params)}>Create one</TextLink>
        </Text>
      )}
    </PageContentBlock>
  )
}
