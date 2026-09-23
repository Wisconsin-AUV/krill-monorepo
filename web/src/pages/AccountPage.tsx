import { useMutation } from '@connectrpc/connect-query'
import { useState } from 'react'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { Description, Field, FieldGroup, Label } from '@/components/ui/Fieldset'
import { Heading, Subheading } from '@/components/ui/Heading'
import { Input } from '@/components/ui/Input'
import PageContentBlock from '@/components/ui/PageContentBlock'
import { Text } from '@/components/ui/Text'
import { AuthService } from '@/gen/krill/v1/auth_pb'
import type { User } from '@/gen/krill/v1/user_pb'
import { refreshSession, roleLabel, roleOptions, useUser } from '@/lib/auth'
import { flash } from '@/lib/flash'

function Section({
  title,
  description,
  children,
}: {
  title: string
  description: React.ReactNode
  children: React.ReactNode
}) {
  return (
    <section className="grid gap-x-8 gap-y-6 border-t border-zinc-950/5 py-10 sm:grid-cols-2 dark:border-white/5">
      <div className="space-y-1">
        <Subheading>{title}</Subheading>
        <Text>{description}</Text>
      </div>
      <div>{children}</div>
    </section>
  )
}

function ProfileForm({ user }: { user: User }) {
  const [form, setForm] = useState({ name: user.name, username: user.username, email: user.email })
  const update = useMutation(AuthService.method.updateProfile, {
    onSuccess: async () => {
      await refreshSession()
      flash.success('Profile saved')
    },
    onError: (err) => flash.error('Could not save profile', err),
  })
  const set = (key: keyof typeof form) => (e: React.ChangeEvent<HTMLInputElement>) =>
    setForm((f) => ({ ...f, [key]: e.target.value }))
  const changed =
    form.name !== user.name || form.username !== user.username || form.email !== user.email

  return (
    <form
      onSubmit={(e) => {
        e.preventDefault()
        update.mutate(form)
      }}
    >
      <FieldGroup>
        <Field>
          <Label>Name</Label>
          <Input value={form.name} onChange={set('name')} required maxLength={100} />
        </Field>
        <Field>
          <Label>Username</Label>
          <Input value={form.username} onChange={set('username')} required maxLength={32} />
        </Field>
        <Field>
          <Label>Email</Label>
          <Input type="email" value={form.email} onChange={set('email')} required />
        </Field>
      </FieldGroup>
      <div className="mt-8 flex justify-end">
        <Button type="submit" color="sky" disabled={!changed || update.isPending}>
          Save
        </Button>
      </div>
    </form>
  )
}

function PasswordForm({ user }: { user: User }) {
  const empty = { current: '', next: '', confirm: '' }
  const [form, setForm] = useState(empty)
  const change = useMutation(AuthService.method.changePassword, {
    onSuccess: async () => {
      setForm(empty)
      await refreshSession()
      flash.success('Password changed', 'Other devices were signed out.')
    },
    onError: (err) => flash.error('Could not change password', err),
  })
  const set = (key: keyof typeof form) => (e: React.ChangeEvent<HTMLInputElement>) =>
    setForm((f) => ({ ...f, [key]: e.target.value }))
  const mismatch = form.confirm !== '' && form.confirm !== form.next

  return (
    <form
      onSubmit={(e) => {
        e.preventDefault()
        change.mutate({ currentPassword: form.current, newPassword: form.next })
      }}
    >
      <FieldGroup>
        {user.hasPassword && (
          <Field>
            <Label>Current password</Label>
            <Input
              type="password"
              value={form.current}
              onChange={set('current')}
              autoComplete="current-password"
              required
            />
          </Field>
        )}
        <Field>
          <Label>New password</Label>
          <Description>At least 8 characters.</Description>
          <Input
            type="password"
            value={form.next}
            onChange={set('next')}
            autoComplete="new-password"
            required
            minLength={8}
          />
        </Field>
        <Field>
          <Label>Confirm new password</Label>
          <Input
            type="password"
            value={form.confirm}
            onChange={set('confirm')}
            autoComplete="new-password"
            required
            invalid={mismatch}
          />
        </Field>
      </FieldGroup>
      <div className="mt-8 flex justify-end">
        <Button type="submit" color="sky" disabled={mismatch || !form.confirm || change.isPending}>
          {user.hasPassword ? 'Change password' : 'Set password'}
        </Button>
      </div>
    </form>
  )
}

export function AccountPage() {
  const user = useUser()
  if (!user) return null

  return (
    <PageContentBlock title="Account · Krill">
      <Heading>Account</Heading>
      <Text className="mt-1">Your profile and how you sign in.</Text>

      <div className="mt-8">
        <Section title="Profile" description="Your username or email works for signing in.">
          <ProfileForm key={user.id + user.name + user.username + user.email} user={user} />
        </Section>
        <Section
          title="Role"
          description={roleOptions.find((o) => o.value === user.role)?.description}
        >
          <div className="flex flex-wrap items-center gap-2">
            <Badge color="sky">{roleLabel(user.role)}</Badge>
            {user.slackLinked && <Badge>Slack linked</Badge>}
          </div>
          <Text className="mt-3">Admins can change your role.</Text>
        </Section>
        <Section
          title="Password"
          description={
            user.hasPassword
              ? 'Changing it signs you out on other devices.'
              : 'You sign in with Slack. Set a password to also sign in with your username.'
          }
        >
          <PasswordForm user={user} />
        </Section>
      </div>
    </PageContentBlock>
  )
}
