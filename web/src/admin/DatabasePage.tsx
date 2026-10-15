import { timestampDate } from '@bufbuild/protobuf/wkt'
import { useMutation, useQuery } from '@connectrpc/connect-query'
import { ArrowDownTrayIcon, PlusIcon, TrashIcon } from '@heroicons/react/16/solid'
import { clsx } from 'clsx'
import { useState } from 'react'
import { ConfirmAlert } from '@/components/ConfirmAlert'
import { Stat } from '@/components/Stat'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { Callout, QueryError } from '@/components/ui/Callout'
import { Heading, Subheading } from '@/components/ui/Heading'
import LoadingSpinner from '@/components/ui/LoadingSpinner'
import PageContentBlock from '@/components/ui/PageContentBlock'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/Table'
import { TextLink } from '@/components/ui/Text'
import { type Backup, type BackupSchedule, DatabaseService } from '@/gen/krill/v1/database_pb'
import { JobStatus } from '@/gen/krill/v1/storage_pb'
import { flash } from '@/lib/flash'
import { formatBytes, formatNumber, formatRelative } from '@/lib/format'
import { invalidateService } from '@/lib/queryClient'
import { isActive } from './jobs'
import { JobStatusBadge } from './JobStatusBadge'

const muted = 'text-zinc-500 dark:text-zinc-400'
const fullDate = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' })

// backups goes between the stats and the tables, which can run long.
function Overview({ backups }: { backups: React.ReactNode }) {
  const { data, isPending, error } = useQuery(DatabaseService.method.getDatabase, {})
  if (isPending) {
    return (
      <div className="flex justify-center py-16">
        <LoadingSpinner />
      </div>
    )
  }
  if (error) {
    return (
      <>
        <QueryError title="Could not load the database" error={error} className="mt-8" />
        {backups}
      </>
    )
  }

  const pending = data.schemaVersion < data.latestSchemaVersion
  return (
    <>
      <div className="mt-8 grid gap-8 sm:grid-cols-3">
        <Stat
          title="Size"
          value={formatBytes(Number(data.sizeBytes))}
          detail={`PostgreSQL ${data.serverVersion}`}
        />
        <Stat
          title="Schema"
          value={String(data.schemaVersion)}
          detail={pending ? `${data.latestSchemaVersion} pending` : 'Up to date'}
        />
        <Stat
          title="River"
          value={String(data.riverVersion)}
          detail={
            data.riverVersion < data.latestRiverVersion
              ? `${data.latestRiverVersion} pending`
              : 'Up to date'
          }
        />
      </div>
      {backups}
      <Subheading className="mt-12">Tables</Subheading>
      <Table dense className="mt-4 [--gutter:--spacing(6)] lg:[--gutter:--spacing(10)]">
        <TableHead>
          <TableRow>
            <TableHeader>Table</TableHeader>
            <TableHeader className="text-right">Rows</TableHeader>
            <TableHeader className="text-right">Indexes</TableHeader>
            <TableHeader className="text-right">Total</TableHeader>
          </TableRow>
        </TableHead>
        <TableBody>
          {data.tables.map((t) => (
            <TableRow key={`${t.schema}.${t.name}`}>
              <TableCell className="font-mono text-xs">
                {t.schema === 'public' ? t.name : `${t.schema}.${t.name}`}
              </TableCell>
              <TableCell className="text-right tabular-nums">
                {t.rowsEstimated && <span className={muted}>~</span>}
                {formatNumber(t.rows)}
              </TableCell>
              <TableCell className={clsx('text-right tabular-nums', muted)}>
                {formatBytes(Number(t.indexBytes))}
              </TableCell>
              <TableCell className="text-right tabular-nums">
                {formatBytes(Number(t.totalBytes))}
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </>
  )
}

function scheduleText(s: BackupSchedule) {
  if (s.hour < 0) return 'No schedule'
  const hour = `${String(s.hour).padStart(2, '0')}:00 ${s.timeZone}`
  return `Daily at ${hour}, keeping ${s.keep}`
}

function Download({ backup }: { backup: Backup }) {
  const download = useMutation(DatabaseService.method.getBackupDownload, {
    onSuccess: (res) => {
      window.location.href = res.url
    },
    onError: (err) => flash.error('Could not download the backup', err),
  })
  return (
    <Button
      plain
      aria-label="Download"
      disabled={download.isPending}
      onClick={() => download.mutate({ id: backup.id })}
    >
      <ArrowDownTrayIcon data-slot="icon" />
    </Button>
  )
}

function Backups() {
  const [deleting, setDeleting] = useState<Backup | null>(null)
  const { data, isPending, error } = useQuery(
    DatabaseService.method.listBackups,
    {},
    {
      refetchInterval: (q) =>
        q.state.data?.backups.some((b) => isActive(b.status)) ? 2000 : false,
    },
  )
  const create = useMutation(DatabaseService.method.createBackup, {
    onSuccess: () => invalidateService(DatabaseService),
    onError: (err) => flash.error('Could not start a backup', err),
  })
  const remove = useMutation(DatabaseService.method.deleteBackup, {
    onSuccess: async () => {
      await invalidateService(DatabaseService)
      setDeleting(null)
    },
  })
  const backups = data?.backups ?? []
  const running = backups.some((b) => isActive(b.status))
  const last = backups.find((b) => b.status === JobStatus.DONE)

  return (
    <section className="mt-12">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <div>
          <Subheading>Backups</Subheading>
          {data && (
            <p className={clsx('text-sm/6', muted)}>
              {last?.createdAt
                ? `Last ${formatRelative(timestampDate(last.createdAt))}`
                : 'None yet'}{' '}
              · {data.schedule && scheduleText(data.schedule)} ·{' '}
              <TextLink to="/admin/settings">Settings</TextLink>
            </p>
          )}
        </div>
        <Button
          outline
          disabled={create.isPending || running || !!data?.unavailable}
          onClick={() => create.mutate({})}
        >
          <PlusIcon data-slot="icon" />
          Back up now
        </Button>
      </div>
      {data?.unavailable && (
        <Callout type="warning" className="mt-4">
          {data.unavailable}
        </Callout>
      )}
      {isPending ? (
        <div className="flex justify-center py-16">
          <LoadingSpinner />
        </div>
      ) : error ? (
        <QueryError title="Could not load backups" error={error} className="mt-4" />
      ) : (
        backups.length > 0 && (
          <Table className="mt-4 [--gutter:--spacing(6)] lg:[--gutter:--spacing(10)]">
            <TableHead>
              <TableRow>
                <TableHeader>Created</TableHeader>
                <TableHeader>By</TableHeader>
                <TableHeader>Status</TableHeader>
                <TableHeader className="text-right">Size</TableHeader>
                <TableHeader>
                  <span className="sr-only">Actions</span>
                </TableHeader>
              </TableRow>
            </TableHead>
            <TableBody>
              {backups.map((b) => {
                const at = b.createdAt && timestampDate(b.createdAt)
                return (
                  <TableRow key={String(b.id)}>
                    <TableCell title={b.filename}>
                      {at && (
                        <>
                          {fullDate.format(at)}
                          <span className={clsx('ml-2', muted)}>{formatRelative(at)}</span>
                        </>
                      )}
                    </TableCell>
                    <TableCell className={muted}>
                      {b.scheduled ? <Badge>Scheduled</Badge> : b.createdBy || '—'}
                    </TableCell>
                    <TableCell>
                      <span className="flex items-center gap-2">
                        <JobStatusBadge status={b.status} />
                        {b.error && (
                          <span
                            className="max-w-xs truncate text-red-700 dark:text-red-400"
                            title={b.error}
                          >
                            {b.error}
                          </span>
                        )}
                      </span>
                    </TableCell>
                    <TableCell className="text-right tabular-nums">
                      {b.status === JobStatus.DONE ? formatBytes(Number(b.sizeBytes)) : ''}
                    </TableCell>
                    <TableCell>
                      <div className="-my-1.5 flex justify-end gap-1">
                        {b.status === JobStatus.DONE && <Download backup={b} />}
                        {!isActive(b.status) && (
                          <Button plain aria-label="Delete" onClick={() => setDeleting(b)}>
                            <TrashIcon data-slot="icon" />
                          </Button>
                        )}
                      </div>
                    </TableCell>
                  </TableRow>
                )
              })}
            </TableBody>
          </Table>
        )
      )}
      <ConfirmAlert
        open={deleting !== null}
        onClose={() => {
          setDeleting(null)
          remove.reset()
        }}
        onConfirm={() => deleting && remove.mutate({ id: deleting.id })}
        title="Delete backup?"
        description={deleting?.filename ?? ''}
        confirmLabel="Delete"
        busy={remove.isPending}
        error={remove.error}
      />
    </section>
  )
}

export function DatabasePage() {
  return (
    <PageContentBlock title="Database · Admin · Krill">
      <Heading>Database</Heading>
      <Overview backups={<Backups />} />
    </PageContentBlock>
  )
}
