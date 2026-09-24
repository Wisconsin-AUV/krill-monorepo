import { useMutation } from '@connectrpc/connect-query'
import { CheckIcon, LockClosedIcon } from '@heroicons/react/16/solid'
import { clsx } from 'clsx'
import { useState } from 'react'
import { PasswordInput } from '@/components/PasswordInput'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { Field, FieldGroup, Label } from '@/components/ui/Fieldset'
import { Heading, Subheading } from '@/components/ui/Heading'
import { Input } from '@/components/ui/Input'
import PageContentBlock from '@/components/ui/PageContentBlock'
import { Text } from '@/components/ui/Text'
import { AuthService } from '@/gen/krill/v1/auth_pb'
import type { PermissionInfo, User } from '@/gen/krill/v1/user_pb'
import { can, refreshSession, useRoleLabel, useSession, useUser } from '@/lib/auth'
import { flash } from '@/lib/flash'
import { usePasswordCheck } from '@/lib/password'

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
          <Input
            name="name"
            autoComplete="name"
            value={form.name}
            onChange={set('name')}
            required
            maxLength={100}
          />
        </Field>
        <Field>
          <Label>Username</Label>
          <Input
            name="username"
            autoComplete="username"
            value={form.username}
            onChange={set('username')}
            required
            maxLength={32}
          />
        </Field>
        <Field>
          <Label>Email</Label>
          <Input
            type="email"
            name="email"
            autoComplete="email"
            value={form.email}
            onChange={set('email')}
            required
          />
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

function RoleSummary({ user, permissions }: { user: User; permissions: PermissionInfo[] }) {
  const roleLabel = useRoleLabel()
  return (
    <div>
      <p className="text-base/7 font-semibold text-zinc-950 sm:text-sm/6 dark:text-white">
        {roleLabel(user.role)}
      </p>
      <ul className="mt-4 space-y-2.5">
        {permissions.map((p) => {
          const allowed = can(user, p.permission)
          return (
            <li
              key={p.permission}
              className={clsx(
                'flex items-center gap-3 text-sm/6',
                allowed ? 'text-zinc-700 dark:text-zinc-300' : 'text-zinc-400 dark:text-zinc-500',
              )}
            >
              {allowed ? (
                <CheckIcon className="size-4 shrink-0 text-emerald-600 dark:text-emerald-400" />
              ) : (
                <LockClosedIcon className="size-4 shrink-0" />
              )}
              <span className="flex-1">{p.description}</span>
              {!allowed && <span className="text-xs/5">{roleLabel(p.role)}</span>}
            </li>
          )
        })}
      </ul>
    </div>
  )
}

function SignInMethods({ user, slackEnabled }: { user: User; slackEnabled: boolean }) {
  const rows = [
    {
      name: 'Password',
      detail: 'Sign in with your username or email.',
      on: user.hasPassword,
      status: user.hasPassword ? 'Set' : 'Not set',
    },
    ...(slackEnabled || user.slackLinked
      ? [
          {
            name: 'Slack',
            detail: user.slackLinked
              ? 'Linked to your Slack account.'
              : 'Use “Continue with Slack” on the sign-in page to link it.',
            on: user.slackLinked,
            status: user.slackLinked ? 'Linked' : 'Not linked',
          },
        ]
      : []),
  ]
  return (
    <ul className="divide-y divide-zinc-950/5 dark:divide-white/5">
      {rows.map((r) => (
        <li key={r.name} className="flex items-center justify-between gap-4 py-3 first:pt-0">
          <div>
            <div className="text-sm/6 font-medium text-zinc-950 dark:text-white">{r.name}</div>
            <div className="text-sm/6 text-zinc-500 dark:text-zinc-400">{r.detail}</div>
          </div>
          <Badge color={r.on ? 'emerald' : 'zinc'}>{r.status}</Badge>
        </li>
      ))}
    </ul>
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
  const set = (key: keyof typeof form) => (value: string) =>
    setForm((f) => ({ ...f, [key]: value }))
  const mismatch = form.confirm !== '' && form.confirm !== form.next
  const check = usePasswordCheck(form.next, [user.name, user.username, user.email])

  return (
    <form
      onSubmit={(e) => {
        e.preventDefault()
        change.mutate({ currentPassword: form.current, newPassword: form.next })
      }}
    >
      <input
        type="text"
        name="username"
        autoComplete="username"
        value={user.username}
        readOnly
        hidden
      />
      <FieldGroup>
        {user.hasPassword && (
          <Field>
            <Label>Current password</Label>
            <PasswordInput value={form.current} onChange={set('current')} name="current-password" />
          </Field>
        )}
        <Field>
          <Label>New password</Label>
          <PasswordInput value={form.next} onChange={set('next')} check={check} />
        </Field>
        <Field>
          <Label>Confirm new password</Label>
          <PasswordInput
            value={form.confirm}
            onChange={set('confirm')}
            name="confirm-password"
            autoComplete="new-password"
            invalid={mismatch}
          />
        </Field>
      </FieldGroup>
      <div className="mt-8 flex justify-end">
        <Button
          type="submit"
          color="sky"
          disabled={mismatch || !form.confirm || !check.ok || change.isPending}
        >
          {user.hasPassword ? 'Change password' : 'Set password'}
        </Button>
      </div>
    </form>
  )
}

export function AccountPage() {
  const { data } = useSession()
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
          description="What you can do in Krill. Ask an admin if you need more access."
        >
          <RoleSummary user={user} permissions={data?.allPermissions ?? []} />
        </Section>
        <Section title="Sign-in methods" description="Ways you can sign in to this account.">
          <SignInMethods user={user} slackEnabled={data?.slackEnabled ?? false} />
        </Section>
        <Section
          title={user.hasPassword ? 'Change password' : 'Set a password'}
          description={
            user.hasPassword
              ? 'Changing it signs you out on other devices.'
              : 'Set a password to sign in with your username or email as well as Slack.'
          }
        >
          <PasswordForm user={user} />
        </Section>
      </div>
    </PageContentBlock>
  )
}
