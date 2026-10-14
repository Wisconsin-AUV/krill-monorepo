import { useMutation, useQuery } from '@connectrpc/connect-query'
import { timestampDate } from '@bufbuild/protobuf/wkt'
import { MagnifyingGlassIcon } from '@heroicons/react/20/solid'
import { useDeferredValue, useState } from 'react'
import { Link } from 'react-router'
import { Avatar } from '@/components/ui/Avatar'
import { Badge } from '@/components/ui/Badge'
import { QueryError } from '@/components/ui/Callout'
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
import { formatRelative } from '@/lib/format'
import { invalidateService } from '@/lib/queryClient'
import { UserActions } from './UserActions'

export function UserBadges({ user, self }: { user: User; self: boolean }) {
  // Unverified emails only matter when sign-up checks them with a code.
  const verifying = useSession().data?.emailVerificationRequired ?? false
  return (
    <>
      {self && <Badge color="sky">You</Badge>}
      {user.disabled && <Badge color="red">Deactivated</Badge>}
      {verifying && !user.emailVerified && <Badge color="amber">Unverified</Badge>}
    </>
  )
}

export function RoleSelect({ user, self }: { user: User; self: boolean }) {
  const roles = useRoles()
  const update = useMutation(UserService.method.updateUser, {
    onSuccess: () => invalidateService(UserService),
    onError: (err) => flash.error('Could not update user', err),
  })
  return (
    <Select
      aria-label={`Role for ${user.name}`}
      value={user.role}
      disabled={self || update.isPending}
      title={self ? 'Another admin has to change your role' : undefined}
      onChange={(e) => update.mutate({ id: user.id, role: Number(e.target.value) as Role })}
    >
      {roles.map((r) => (
        <option key={r.role} value={r.role}>
          {r.label}
        </option>
      ))}
    </Select>
  )
}

export function UsersPage() {
  const me = useUser()
  const { data, isPending, error } = useQuery(UserService.method.listUsers, {})
  const [search, setSearch] = useState('')
  const query = useDeferredValue(search.trim().toLowerCase())
  const all = data?.users ?? []
  const users = all.filter(
    (u) =>
      !query ||
      u.name.toLowerCase().includes(query) ||
      u.username.includes(query) ||
      u.email.includes(query),
  )

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
                    <Link to={`/admin/users/${u.id}`} className="group flex items-center gap-3">
                      <Avatar seed={u.id} className={u.disabled ? 'size-8 grayscale' : 'size-8'} />
                      <div className="min-w-0">
                        <div className="flex items-center gap-2 font-medium text-zinc-950 group-hover:underline dark:text-white">
                          {u.name}
                          <UserBadges user={u} self={self} />
                        </div>
                        <div className="truncate text-zinc-500 dark:text-zinc-400">{u.email}</div>
                      </div>
                    </Link>
                  </TableCell>
                  <TableCell className="text-zinc-500 dark:text-zinc-400">@{u.username}</TableCell>
                  <TableCell>
                    <RoleSelect user={u} self={self} />
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
                  <TableCell>{!self && <UserActions user={u} users={all} />}</TableCell>
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
    </PageContentBlock>
  )
}
