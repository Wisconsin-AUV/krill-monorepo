import { useMutation } from '@connectrpc/connect-query'
import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router'
import { CODE_LENGTH, CodeInput } from '@/components/auth/CodeInput'
import { Button } from '@/components/ui/Button'
import { FormError } from '@/components/ui/Callout'
import { Field, Label } from '@/components/ui/Fieldset'
import { Heading } from '@/components/ui/Heading'
import { Input } from '@/components/ui/Input'
import PageContentBlock from '@/components/ui/PageContentBlock'
import { Strong, Text, TextButton } from '@/components/ui/Text'
import { AuthService } from '@/gen/krill/v1/auth_pb'
import { refreshSession, useUser } from '@/lib/auth'
import { flash } from '@/lib/flash'
import { queryClient } from '@/lib/queryClient'

export function VerifyEmailPage() {
  const user = useUser()
  const navigate = useNavigate()
  const [code, setCode] = useState('')
  const [editing, setEditing] = useState(false)
  const [email, setEmail] = useState('')

  const send = useMutation(AuthService.method.sendVerificationCode)
  const resend = useMutation(AuthService.method.sendVerificationCode, {
    onSuccess: () => flash.success('Code sent'),
  })
  const verify = useMutation(AuthService.method.verifyEmail, {
    onSuccess: () => refreshSession(),
    onError: () => setCode(''),
  })
  const update = useMutation(AuthService.method.updateProfile, {
    onSuccess: async () => {
      setEditing(false)
      setCode('')
      await refreshSession()
    },
  })
  const logout = useMutation(AuthService.method.logout, {
    onSuccess: () => {
      queryClient.clear()
      navigate('/login')
    },
  })

  // Sends a code only when there is no live one, so reloading the page
  // doesn't invalidate the code already in the inbox.
  const { mutate: sendCode } = send
  useEffect(() => {
    sendCode({})
  }, [sendCode])

  if (!user) return null
  const target = user.pendingEmail || user.email

  if (editing) {
    return (
      <PageContentBlock title="Verify your email · Krill" width="w-full max-w-sm">
        <form
          className="grid grid-cols-1 gap-8"
          onSubmit={(e) => {
            e.preventDefault()
            update.mutate({ name: user.name, username: user.username, email })
          }}
        >
          <Heading className="text-center">Change your email</Heading>
          <FormError error={update.error} />
          <Field>
            <Label>Email</Label>
            <Input
              type="email"
              name="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              autoComplete="email"
              required
            />
          </Field>
          <Button type="submit" color="sky" className="w-full" disabled={update.isPending}>
            Send code
          </Button>
          <TextButton onClick={() => setEditing(false)}>Back</TextButton>
        </form>
      </PageContentBlock>
    )
  }

  return (
    <PageContentBlock title="Verify your email · Krill" width="w-full max-w-sm">
      <form
        className="grid grid-cols-1 gap-8"
        onSubmit={(e) => {
          e.preventDefault()
          verify.mutate({ code })
        }}
      >
        <div className="space-y-2 text-center">
          <Heading>Verify your email</Heading>
          <Text>
            Enter the code sent to <Strong>{target}</Strong>.
          </Text>
        </div>
        <FormError error={verify.error ?? resend.error ?? send.error} />
        <Field>
          <Label>Code</Label>
          <CodeInput
            value={code}
            onChange={(value) => {
              setCode(value)
              if (value.length === CODE_LENGTH && !verify.isPending) verify.mutate({ code: value })
            }}
          />
        </Field>
        <Button
          type="submit"
          color="sky"
          className="w-full"
          disabled={code.length !== CODE_LENGTH || verify.isPending}
        >
          Verify
        </Button>
        <div className="flex justify-between">
          <TextButton onClick={() => resend.mutate({ resend: true })} disabled={resend.isPending}>
            Send a new code
          </TextButton>
          <TextButton
            onClick={() => {
              setEmail(target)
              setEditing(true)
            }}
          >
            Change email
          </TextButton>
        </div>
        <TextButton onClick={() => logout.mutate({})} disabled={logout.isPending}>
          Sign out
        </TextButton>
      </form>
    </PageContentBlock>
  )
}
