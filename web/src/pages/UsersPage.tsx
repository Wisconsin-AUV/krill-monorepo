import { useMutation, useQuery } from '@connectrpc/connect-query'
import { timestampDate } from '@bufbuild/protobuf/wkt'
import {
  EllipsisHorizontalIcon,
  KeyIcon,
  LockClosedIcon,
  LockOpenIcon,
  TrashIcon,
} from '@heroicons/react/20/solid'
import { useState } from 'react'
import { ConfirmAlert } from '@/components/ConfirmAlert'
import { PasswordInput } from '@/components/PasswordInput'
import { Avatar } from '@/components/ui/Avatar'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
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
import { Text } from '@/components/ui/Text'
import { UserService, type Role, type User } from '@/gen/krill/v1/user_pb'
import { initials, useRoles, useUser } from '@/lib/auth'
import { errorMessage } from '@/lib/errors'
import { flash } from '@/lib/flash'
import { formatRelative } from '@/lib/format'
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
    onError: (err) => flash.error('Could not set password', err),
  })

  return (
    <Dialog open={user !== null} onClose={onClose} size="md">
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
        </DialogBody>
        <DialogActions>
          <Button plain onClick={onClose}>
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
  const { data, isPending, error } = useQuery(UserService.method.listUsers, {})
  const [settingPassword, setSettingPassword] = useState<User | null>(null)
  const [deleting, setDeleting] = useState<User | null>(null)
  const users = data?.users ?? []

  const update = useMutation(UserService.method.updateUser, {
    onSuccess: () => invalidateService(UserService),
    onError: (err) => flash.error('Could not update user', err),
  })
  const remove = useMutation(UserService.method.deleteUser, {
    onSuccess: async () => {
      await invalidateService(UserService)
      setDeleting(null)
    },
    onError: (err) => {
      setDeleting(null)
      flash.error('Could not delete user', err)
    },
  })

  return (
    <PageContentBlock title="Users · Krill">
      <Heading>Users</Heading>
      <Text className="mt-1">
        Each role has everything the role below it has. Everyone can see what their role allows on
        their Account page.
      </Text>

      {isPending ? (
        <div className="flex justify-center py-16">
          <LoadingSpinner />
        </div>
      ) : error ? (
        <Text className="mt-8 text-red-600 dark:text-red-400">
          Could not load users: {errorMessage(error)}
        </Text>
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
                      <Avatar
                        initials={initials(u.name)}
                        className={
                          u.disabled
                            ? 'size-8 rounded-full bg-zinc-400'
                            : 'size-8 rounded-full bg-sky-600'
                        }
                      />
                      <div className="min-w-0">
                        <div className="flex items-center gap-2 font-medium text-zinc-950 dark:text-white">
                          {u.name}
                          {self && <Badge color="sky">You</Badge>}
                          {u.disabled && <Badge color="red">Disabled</Badge>}
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

      <PasswordDialog user={settingPassword} onClose={() => setSettingPassword(null)} />
      <ConfirmAlert
        open={deleting !== null}
        onClose={() => setDeleting(null)}
        onConfirm={() => deleting && remove.mutate({ id: deleting.id })}
        busy={remove.isPending}
        title={`Delete ${deleting?.name ?? ''}?`}
        description="Their labels stay. Disable the account instead if they might come back."
        confirmLabel="Delete user"
      />
    </PageContentBlock>
  )
}
