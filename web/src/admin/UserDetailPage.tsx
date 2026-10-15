import { timestampDate, type Timestamp } from '@bufbuild/protobuf/wkt'
import { useQuery } from '@connectrpc/connect-query'
import { ChevronLeftIcon } from '@heroicons/react/16/solid'
import { useNavigate, useParams } from 'react-router'
import { SessionsTable } from '@/components/SessionsTable'
import { Avatar } from '@/components/ui/Avatar'
import { Badge } from '@/components/ui/Badge'
import { QueryError } from '@/components/ui/Callout'
import {
  DescriptionDetails,
  DescriptionList,
  DescriptionTerm,
} from '@/components/ui/DescriptionList'
import { Heading, Subheading } from '@/components/ui/Heading'
import LoadingSpinner from '@/components/ui/LoadingSpinner'
import PageContentBlock from '@/components/ui/PageContentBlock'
import { TextLink } from '@/components/ui/Text'
import { AuditService } from '@/gen/krill/v1/audit_pb'
import { SessionService } from '@/gen/krill/v1/session_pb'
import { Permission, UserService, type User } from '@/gen/krill/v1/user_pb'
import { useCan, useUser } from '@/lib/auth'
import { summarize } from '@/lib/audit'
import { formatRelative } from '@/lib/format'
import { useRevokeSession } from '@/lib/sessions'
import { UserActions } from './UserActions'
import { RoleSelect, UserBadges } from './UsersPage'

const fullDate = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' })

function When({ at, empty = 'Never' }: { at?: Timestamp; empty?: string }) {
  if (!at) return <>{empty}</>
  const date = timestampDate(at)
  return <span title={fullDate.format(date)}>{formatRelative(date)}</span>
}

function Sessions({ user }: { user: User }) {
  const { data, isPending, error } = useQuery(SessionService.method.listSessions, {
    userId: user.id,
  })
  const revoke = useRevokeSession()
  if (isPending) return <LoadingSpinner className="mt-6 size-6 text-gray-500" />
  if (error) return <QueryError title="Could not load sessions" error={error} className="mt-6" />
  if (data.sessions.length === 0) {
    return <p className="mt-4 text-sm/6 text-zinc-500 dark:text-zinc-400">Not signed in</p>
  }
  return (
    <SessionsTable
      sessions={data.sessions}
      onRevoke={(s) => revoke.mutate({ id: s.id })}
      revoking={revoke.isPending ? revoke.variables?.id : undefined}
      className="mt-4"
    />
  )
}

function Activity({ user }: { user: User }) {
  const { data, isPending, error } = useQuery(AuditService.method.listAuditLog, {
    targetType: 'user',
    targetId: user.id,
    pageSize: 20,
  })
  if (isPending) return <LoadingSpinner className="mt-6 size-6 text-gray-500" />
  if (error) return <QueryError title="Could not load activity" error={error} className="mt-6" />
  if (data.entries.length === 0) {
    return <p className="mt-4 text-sm/6 text-zinc-500 dark:text-zinc-400">No changes yet</p>
  }
  return (
    <ul className="mt-4 divide-y divide-zinc-950/5 text-sm/6 dark:divide-white/5">
      {data.entries.map((e) => (
        <li key={String(e.id)} className="flex flex-wrap items-baseline gap-x-3 py-2.5">
          <span className="font-mono text-xs text-zinc-950 dark:text-white">{e.action}</span>
          <span className="min-w-0 flex-1 truncate text-zinc-500 dark:text-zinc-400">
            {summarize(e.details)}
          </span>
          <span className="text-zinc-500 dark:text-zinc-400">
            {e.actor?.name ?? (e.actorName || 'System')} · <When at={e.at} />
          </span>
        </li>
      ))}
    </ul>
  )
}

export function UserDetailPage() {
  const { id } = useParams()
  const me = useUser()
  const navigate = useNavigate()
  const canAudit = useCan(Permission.MANAGE_SYSTEM)
  const { data, isPending, error } = useQuery(UserService.method.listUsers, {})
  const user = data?.users.find((u) => u.id === id)
  const self = user?.id === me?.id

  return (
    <PageContentBlock title={`${user?.name ?? 'User'} · Admin · Krill`}>
      <TextLink
        to="/admin/users"
        className="inline-flex items-center gap-1 text-sm/6 text-zinc-500 no-underline dark:text-zinc-400"
      >
        <ChevronLeftIcon className="size-4 fill-zinc-400" />
        Users
      </TextLink>

      {isPending ? (
        <div className="flex justify-center py-16">
          <LoadingSpinner />
        </div>
      ) : error ? (
        <QueryError title="Could not load the user" error={error} className="mt-8" />
      ) : !user ? (
        <p className="py-16 text-center text-sm/6 text-zinc-500 dark:text-zinc-400">
          User not found
        </p>
      ) : (
        <>
          <div className="mt-4 flex flex-wrap items-center justify-between gap-4">
            <div className="flex min-w-0 items-center gap-4">
              <Avatar seed={user.id} className={user.disabled ? 'size-12 grayscale' : 'size-12'} />
              <div className="min-w-0">
                <div className="flex flex-wrap items-center gap-2">
                  <Heading>{user.name}</Heading>
                  <UserBadges user={user} self={self} />
                </div>
                <p className="truncate text-sm/6 text-zinc-500 dark:text-zinc-400">
                  @{user.username} · {user.email}
                </p>
              </div>
            </div>
            <div className="flex items-center gap-2">
              <div className="w-36">
                <RoleSelect user={user} self={self} />
              </div>
              {!self && (
                <UserActions
                  user={user}
                  users={data.users}
                  onDeleted={() => void navigate('/admin/users')}
                />
              )}
            </div>
          </div>

          <DescriptionList className="mt-10">
            <DescriptionTerm>Joined</DescriptionTerm>
            <DescriptionDetails>
              <When at={user.createdAt} />
            </DescriptionDetails>
            <DescriptionTerm>Last sign-in</DescriptionTerm>
            <DescriptionDetails>
              <When at={user.lastLoginAt} />
            </DescriptionDetails>
            <DescriptionTerm>Sign-in</DescriptionTerm>
            <DescriptionDetails>
              <span className="flex flex-wrap gap-1.5">
                {user.hasPassword && <Badge>Password</Badge>}
                {user.slackLinked && <Badge>Slack</Badge>}
                {!user.hasPassword && !user.slackLinked && '—'}
              </span>
            </DescriptionDetails>
            {user.pendingEmail && (
              <>
                <DescriptionTerm>Pending email</DescriptionTerm>
                <DescriptionDetails>{user.pendingEmail}</DescriptionDetails>
              </>
            )}
          </DescriptionList>

          <section className="mt-12">
            <Subheading>Sessions</Subheading>
            <Sessions user={user} />
          </section>

          {canAudit && (
            <section className="mt-12">
              <Subheading>Activity</Subheading>
              <Activity user={user} />
            </section>
          )}
        </>
      )}
    </PageContentBlock>
  )
}
