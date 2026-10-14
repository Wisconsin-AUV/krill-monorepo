import { useMutation, useQuery } from '@connectrpc/connect-query'
import { timestampDate } from '@bufbuild/protobuf/wkt'
import {
  ArrowRightStartOnRectangleIcon,
  CheckBadgeIcon,
  EllipsisHorizontalIcon,
  KeyIcon,
  LockClosedIcon,
  LockOpenIcon,
  MagnifyingGlassIcon,
  TrashIcon,
} from '@heroicons/react/20/solid'
import { useDeferredValue, useState } from 'react'
import { ConfirmAlert } from '@/components/ConfirmAlert'
import { PasswordInput } from '@/components/PasswordInput'
import { Avatar } from '@/components/ui/Avatar'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { FormError, QueryError } from '@/components/ui/Callout'
import { Dialog, DialogActions, DialogBody, DialogTitle } from '@/components/ui/Dialog'
import {
  Dropdown,
  DropdownButton,
  DropdownItem,
  DropdownLabel,
  DropdownMenu,
} from '@/components/ui/Dropdown'
import { Description, Field, Label } from '@/components/ui/Fieldset'
import { Heading } from '@/components/ui/Heading'
import { Input, InputGroup } from '@/components/ui/Input'
import LoadingSpinner from '@/components/ui/LoadingSpinner'
import PageContentBlock from '@/components/ui/PageContentBlock'
import { Select } from '@/components/ui/Select'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/Table'
import { UserService, type Role, type User } from '@/gen/krill/v1/user_pb'
import { useRoles, useSession, useUser } from '@/lib/auth'
import { flash } from '@/lib/flash'
import { formatRelative, plural } from '@/lib/format'
import { usePasswordCheck } from '@/lib/password'
import { invalidateService } from '@/lib/queryClient'

function PasswordDialog({ user, onClose }: { user: User | null; onClose: () => void }) {
  const [password, setPassword] = useState('')
  const check = usePasswordCheck(password, user ? [user.name, user.username, user.email] : [])
  const set = useMutation(UserService.method.setUserPassword, {
    onSuccess: () => {
      flash.success('Password set', `${user?.name} was signed out everywhere.`)
      setPassword('')
      onClose()
    },
  })
  const close = () => {
    set.reset()
    onClose()
  }

  return (
    <Dialog open={user !== null} onClose={close} size="md">
      <form
        onSubmit={(e) => {
          e.preventDefault()
          if (user) set.mutate({ id: user.id, password })
        }}
      >
        <input
          type="text"
          name="username"
          autoComplete="username"
          value={user?.username ?? ''}
          readOnly
          hidden
        />
        <DialogTitle>Set password for {user?.name}</DialogTitle>
        <DialogBody>
          <Field>
            <Label>New password</Label>
            <Description>Share it with them directly. They can change it later.</Description>
            <PasswordInput value={password} onChange={setPassword} check={check} />
          </Field>
          <FormError error={set.error} className="mt-6" />
        </DialogBody>
        <DialogActions>
          <Button plain onClick={close}>
            Cancel
          </Button>
          <Button type="submit" color="sky" disabled={set.isPending || !check.ok}>
            Set password
          </Button>
        </DialogActions>
      </form>
    </Dialog>
  )
}

export function UsersPage() {
  const me = useUser()
  const roles = useRoles()
  // Unverified emails only matter when sign-up checks them with a code.
  const verifying = useSession().data?.emailVerificationRequired ?? false
  const { data, isPending, error } = useQuery(UserService.method.listUsers, {})
  const [settingPassword, setSettingPassword] = useState<User | null>(null)
  const [deleting, setDeleting] = useState<User | null>(null)
  const [search, setSearch] = useState('')
  const query = useDeferredValue(search.trim().toLowerCase())
  const users = (data?.users ?? []).filter(
    (u) =>
      !query ||
      u.name.toLowerCase().includes(query) ||
      u.username.includes(query) ||
      u.email.includes(query),
  )

  const update = useMutation(UserService.method.updateUser, {
    onSuccess: () => invalidateService(UserService),
    onError: (err) => flash.error('Could not update user', err),
  })
  const signOut = useMutation(UserService.method.signOutUser, {
    onSuccess: async (_, req) => {
      await invalidateService(UserService)
      const u = data?.users.find((x) => x.id === req.id)
      flash.success('Signed out', `${u?.name ?? 'They'} will need to sign in again.`)
    },
    onError: (err) => flash.error('Could not sign out user', err),
  })
  const verify = useMutation(UserService.method.markEmailVerified, {
    onSuccess: () => invalidateService(UserService),
    onError: (err) => flash.error('Could not verify email', err),
  })
  const remove = useMutation(UserService.method.deleteUser, {
    onSuccess: async () => {
      await invalidateService(UserService)
      setDeleting(null)
    },
  })

  return (
    <PageContentBlock title="Users · Admin · Krill">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <Heading>Users</Heading>
        <div className="w-full sm:max-w-xs">
          <InputGroup>
            <MagnifyingGlassIcon data-slot="icon" />
            <Input
              type="search"
              aria-label="Search users"
              placeholder="Search"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
            />
          </InputGroup>
        </div>
      </div>

      {isPending ? (
        <div className="flex justify-center py-16">
          <LoadingSpinner />
        </div>
      ) : error ? (
        <QueryError title="Could not load users" error={error} className="mt-8" />
      ) : (
        <Table className="mt-8 [--gutter:--spacing(6)] lg:[--gutter:--spacing(10)]">
          <TableHead>
            <TableRow>
              <TableHeader>User</TableHeader>
              <TableHeader>Username</TableHeader>
              <TableHeader>Role</TableHeader>
              <TableHeader>Sign-in</TableHeader>
              <TableHeader className="text-right">Last sign-in</TableHeader>
              <TableHeader className="relative w-0">
                <span className="sr-only">Actions</span>
              </TableHeader>
            </TableRow>
          </TableHead>
          <TableBody>
            {users.map((u) => {
              const self = u.id === me?.id
              return (
                <TableRow key={u.id}>
                  <TableCell>
                    <div className="flex items-center gap-3">
                      <Avatar seed={u.id} className={u.disabled ? 'size-8 grayscale' : 'size-8'} />
                      <div className="min-w-0">
                        <div className="flex items-center gap-2 font-medium text-zinc-950 dark:text-white">
                          {u.name}
                          {self && <Badge color="sky">You</Badge>}
                          {u.disabled && <Badge color="red">Disabled</Badge>}
                          {verifying && !u.emailVerified && <Badge color="amber">Unverified</Badge>}
                        </div>
                        <div className="truncate text-zinc-500 dark:text-zinc-400">{u.email}</div>
                      </div>
                    </div>
                  </TableCell>
                  <TableCell className="text-zinc-500 dark:text-zinc-400">@{u.username}</TableCell>
                  <TableCell>
                    <Select
                      aria-label={`Role for ${u.name}`}
                      value={u.role}
                      disabled={self || update.isPending}
                      title={self ? 'Another admin has to change your role' : undefined}
                      onChange={(e) =>
                        update.mutate({ id: u.id, role: Number(e.target.value) as Role })
                      }
                    >
                      {roles.map((r) => (
                        <option key={r.role} value={r.role}>
                          {r.label}
                        </option>
                      ))}
                    </Select>
                  </TableCell>
                  <TableCell>
                    <div className="flex flex-wrap gap-1.5">
                      {u.hasPassword && <Badge>Password</Badge>}
                      {u.slackLinked && <Badge>Slack</Badge>}
                    </div>
                  </TableCell>
                  <TableCell className="text-right text-zinc-500 dark:text-zinc-400">
                    {u.lastLoginAt ? formatRelative(timestampDate(u.lastLoginAt)) : 'Never'}
                  </TableCell>
                  <TableCell>
                    {!self && (
                      <Dropdown>
                        <DropdownButton plain aria-label={`Options for ${u.name}`}>
                          <EllipsisHorizontalIcon data-slot="icon" />
                        </DropdownButton>
                        <DropdownMenu anchor="bottom end">
                          <DropdownItem onClick={() => setSettingPassword(u)}>
                            <KeyIcon data-slot="icon" />
                            <DropdownLabel>Set password</DropdownLabel>
                          </DropdownItem>
                          {verifying && !u.emailVerified && (
                            <DropdownItem onClick={() => verify.mutate({ id: u.id })}>
                              <CheckBadgeIcon data-slot="icon" />
                              <DropdownLabel>Mark email verified</DropdownLabel>
                            </DropdownItem>
                          )}
                          <DropdownItem
                            onClick={() => signOut.mutate({ id: u.id })}
                            disabled={u.activeSessions === 0}
                          >
                            <ArrowRightStartOnRectangleIcon data-slot="icon" />
                            <DropdownLabel>
                              {u.activeSessions > 0
                                ? `Sign out of ${plural(u.activeSessions, 'session')}`
                                : 'Not signed in'}
                            </DropdownLabel>
                          </DropdownItem>
                          <DropdownItem
                            onClick={() => update.mutate({ id: u.id, disabled: !u.disabled })}
                          >
                            {u.disabled ? (
                              <LockOpenIcon data-slot="icon" />
                            ) : (
                              <LockClosedIcon data-slot="icon" />
                            )}
                            <DropdownLabel>{u.disabled ? 'Enable' : 'Disable'}</DropdownLabel>
                          </DropdownItem>
                          <DropdownItem onClick={() => setDeleting(u)}>
                            <TrashIcon data-slot="icon" />
                            <DropdownLabel>Delete</DropdownLabel>
                          </DropdownItem>
                        </DropdownMenu>
                      </Dropdown>
                    )}
                  </TableCell>
                </TableRow>
              )
            })}
          </TableBody>
        </Table>
      )}
      {!isPending && !error && users.length === 0 && query && (
        <p className="py-10 text-center text-sm/6 text-zinc-500 dark:text-zinc-400">
          No users match “{search.trim()}”
        </p>
      )}

      <PasswordDialog user={settingPassword} onClose={() => setSettingPassword(null)} />
      <ConfirmAlert
        open={deleting !== null}
        onClose={() => {
          setDeleting(null)
          remove.reset()
        }}
        onConfirm={() => deleting && remove.mutate({ id: deleting.id })}
        busy={remove.isPending}
        error={remove.error}
        title={`Delete ${deleting?.name ?? ''}?`}
        description="Their labels stay. Disable the account instead if they might come back."
        confirmLabel="Delete user"
      />
    </PageContentBlock>
  )
}
