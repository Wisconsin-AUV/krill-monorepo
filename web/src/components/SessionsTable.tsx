import { timestampDate, type Timestamp } from '@bufbuild/protobuf/wkt'
import { ComputerDesktopIcon, DevicePhoneMobileIcon } from '@heroicons/react/20/solid'
import { Link } from 'react-router'
import { Avatar } from '@/components/ui/Avatar'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/Table'
import type { Session } from '@/gen/krill/v1/session_pb'
import { formatRelative } from '@/lib/format'
import { isActive } from '@/lib/sessions'

const fullDate = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' })

function When({ at }: { at?: Timestamp }) {
  if (!at) return null
  const date = timestampDate(at)
  return <span title={fullDate.format(date)}>{formatRelative(date)}</span>
}

function DeviceIcon({ device }: { device: string }) {
  const Icon = /iOS|Android/.test(device) ? DevicePhoneMobileIcon : ComputerDesktopIcon
  return <Icon className="size-5 shrink-0 fill-zinc-400 dark:fill-zinc-500" aria-hidden />
}

export function SessionsTable({
  sessions,
  showUser = false,
  onRevoke,
  revoking,
  className,
}: {
  sessions: Session[]
  showUser?: boolean
  onRevoke: (s: Session) => void
  revoking?: string
  className?: string
}) {
  return (
    <Table className={className}>
      <TableHead>
        <TableRow>
          {showUser && <TableHeader>User</TableHeader>}
          <TableHeader>Device</TableHeader>
          <TableHeader>IP</TableHeader>
          <TableHeader>Signed in</TableHeader>
          <TableHeader>Last active</TableHeader>
          <TableHeader className="relative w-0">
            <span className="sr-only">Actions</span>
          </TableHeader>
        </TableRow>
      </TableHead>
      <TableBody>
        {sessions.map((s) => (
          <TableRow key={s.id}>
            {showUser && (
              <TableCell>
                {s.user && (
                  <Link to={`/admin/users/${s.user.id}`} className="flex items-center gap-2">
                    <Avatar seed={s.user.id} className="size-6" />
                    <span className="font-medium text-zinc-950 hover:underline dark:text-white">
                      {s.user.name}
                    </span>
                  </Link>
                )}
              </TableCell>
            )}
            <TableCell>
              <span className="flex items-center gap-2" title={s.userAgent || undefined}>
                <DeviceIcon device={s.device} />
                <span className="text-zinc-950 dark:text-white">{s.device}</span>
                {s.current && <Badge color="sky">This device</Badge>}
              </span>
            </TableCell>
            <TableCell className="font-mono text-xs text-zinc-500 dark:text-zinc-400">
              {s.ip || '—'}
            </TableCell>
            <TableCell className="text-zinc-500 dark:text-zinc-400">
              <When at={s.createdAt} />
            </TableCell>
            <TableCell className="text-zinc-500 dark:text-zinc-400">
              {isActive(s) ? (
                <span className="flex items-center gap-2">
                  <span className="size-1.5 rounded-full bg-emerald-500" aria-hidden />
                  Active now
                </span>
              ) : (
                <When at={s.lastSeenAt} />
              )}
            </TableCell>
            <TableCell className="text-right">
              {!s.current && (
                <Button plain disabled={revoking === s.id} onClick={() => onRevoke(s)}>
                  Revoke
                </Button>
              )}
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  )
}
