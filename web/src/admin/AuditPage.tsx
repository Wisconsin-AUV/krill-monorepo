import type { JsonObject, JsonValue } from '@bufbuild/protobuf'
import { timestampDate } from '@bufbuild/protobuf/wkt'
import { useInfiniteQuery, useQuery } from '@connectrpc/connect-query'
import { useState } from 'react'
import { Avatar } from '@/components/ui/Avatar'
import { Button } from '@/components/ui/Button'
import { QueryError } from '@/components/ui/Callout'
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
import { AuditService, type AuditEntry } from '@/gen/krill/v1/audit_pb'
import { formatRelative } from '@/lib/format'

const fullDate = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'medium' })

function show(v: JsonValue | undefined): string {
  if (v === null || v === undefined || v === '') return '—'
  if (Array.isArray(v)) return v.map(show).join(', ')
  if (typeof v === 'object') return JSON.stringify(v)
  return String(v)
}

function summarize(details: JsonObject | undefined): string {
  if (!details) return ''
  const parts: string[] = []
  if ('from' in details || 'to' in details)
    parts.push(`${show(details.from)} → ${show(details.to)}`)
  for (const [key, value] of Object.entries(details)) {
    if (key === 'from' || key === 'to') continue
    if (value === true) parts.push(key)
    else parts.push(`${key}: ${show(value)}`)
  }
  return parts.join(' · ')
}

// Each group's prefix option lists every action in it.
function actionGroups(actions: string[]) {
  const groups = new Map<string, string[]>()
  for (const a of actions) {
    const group = a.split('.')[0]
    groups.set(group, [...(groups.get(group) ?? []), a])
  }
  return [...groups]
}

function Actor({ entry }: { entry: AuditEntry }) {
  if (!entry.actor) {
    return <span className="text-zinc-500 dark:text-zinc-400">{entry.actorName || 'System'}</span>
  }
  return (
    <span className="flex items-center gap-2">
      <Avatar seed={entry.actor.id} className="size-6" />
      <span className="font-medium text-zinc-950 dark:text-white">{entry.actor.name}</span>
    </span>
  )
}

export function AuditPage() {
  const [actorId, setActorId] = useState('')
  const [actionPrefix, setActionPrefix] = useState('')
  const filters = useQuery(AuditService.method.listAuditFilters, {})
  const { data, isPending, error, fetchNextPage, hasNextPage, isFetchingNextPage } =
    useInfiniteQuery(
      AuditService.method.listAuditLog,
      { actorId, actionPrefix, pageToken: '' },
      { pageParamKey: 'pageToken', getNextPageParam: (p) => p.nextPageToken || undefined },
    )
  const entries = data?.pages.flatMap((p) => p.entries) ?? []
  const filtered = actorId !== '' || actionPrefix !== ''

  return (
    <PageContentBlock title="Audit log · Admin · Krill">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <Heading>Audit log</Heading>
        <div className="flex w-full gap-3 sm:w-auto">
          <Select
            aria-label="Actor"
            value={actorId}
            onChange={(e) => setActorId(e.target.value)}
            className="sm:w-48"
          >
            <option value="">Everyone</option>
            {filters.data?.actors.map((a) => (
              <option key={a.id} value={a.id}>
                {a.name}
              </option>
            ))}
          </Select>
          <Select
            aria-label="Action"
            value={actionPrefix}
            onChange={(e) => setActionPrefix(e.target.value)}
            className="sm:w-48"
          >
            <option value="">All actions</option>
            {actionGroups(filters.data?.actions ?? []).map(([group, actions]) => (
              <optgroup key={group} label={group}>
                <option value={`${group}.`}>{group}.*</option>
                {actions.map((a) => (
                  <option key={a} value={a}>
                    {a}
                  </option>
                ))}
              </optgroup>
            ))}
          </Select>
        </div>
      </div>

      {isPending ? (
        <div className="flex justify-center py-16">
          <LoadingSpinner />
        </div>
      ) : error ? (
        <QueryError title="Could not load the audit log" error={error} className="mt-8" />
      ) : entries.length === 0 ? (
        <p className="py-10 text-center text-sm/6 text-zinc-500 dark:text-zinc-400">
          {filtered ? 'No matching entries' : 'No entries yet'}
        </p>
      ) : (
        <Table className="mt-8 [--gutter:--spacing(6)] lg:[--gutter:--spacing(10)]">
          <TableHead>
            <TableRow>
              <TableHeader>Time</TableHeader>
              <TableHeader>Actor</TableHeader>
              <TableHeader>Action</TableHeader>
              <TableHeader>Target</TableHeader>
              <TableHeader>Details</TableHeader>
            </TableRow>
          </TableHead>
          <TableBody>
            {entries.map((e) => {
              const at = e.at ? timestampDate(e.at) : null
              return (
                <TableRow key={String(e.id)}>
                  <TableCell
                    className="whitespace-nowrap text-zinc-500 dark:text-zinc-400"
                    title={at ? `${fullDate.format(at)}${e.ip ? ` · ${e.ip}` : ''}` : undefined}
                  >
                    {at ? formatRelative(at) : ''}
                  </TableCell>
                  <TableCell>
                    <Actor entry={e} />
                  </TableCell>
                  <TableCell className="font-mono text-xs">{e.action}</TableCell>
                  <TableCell>
                    {e.targetName || e.targetId}
                    {e.targetType && (
                      <span className="ml-2 text-zinc-500 dark:text-zinc-400">
                        {e.targetType.replace('_', ' ')}
                      </span>
                    )}
                  </TableCell>
                  <TableCell className="max-w-sm truncate text-zinc-500 dark:text-zinc-400">
                    {summarize(e.details)}
                  </TableCell>
                </TableRow>
              )
            })}
          </TableBody>
        </Table>
      )}
      {hasNextPage && (
        <div className="mt-6 flex justify-center">
          <Button outline disabled={isFetchingNextPage} onClick={() => void fetchNextPage()}>
            Load more
          </Button>
        </div>
      )}
    </PageContentBlock>
  )
}
