import { timestampDate } from '@bufbuild/protobuf/wkt'
import { useMutation, useQuery } from '@connectrpc/connect-query'
import { ArrowPathIcon, ChevronDownIcon, ChevronUpIcon } from '@heroicons/react/16/solid'
import { ArchiveBoxIcon } from '@heroicons/react/20/solid'
import { clsx } from 'clsx'
import { useState } from 'react'
import { ConfirmAlert } from '@/components/ConfirmAlert'
import { EmptyState } from '@/components/EmptyState'
import { Meter } from '@/components/Meter'
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
import {
  JobStatus,
  ObjectKind,
  type ObjectCategory,
  type Orphan,
  type StorageScan,
  StorageService,
  type VideoUsage,
} from '@/gen/krill/v1/storage_pb'
import { flash } from '@/lib/flash'
import { formatBytes, formatNumber, formatRelative, plural } from '@/lib/format'
import { invalidateService } from '@/lib/queryClient'
import { isActive } from './jobs'
import { JobStatusBadge } from './JobStatusBadge'

const kindNames: Record<ObjectKind, string> = {
  [ObjectKind.UNSPECIFIED]: 'Unknown',
  [ObjectKind.SOURCE]: 'Videos',
  [ObjectKind.FRAME]: 'Frames',
  [ObjectKind.EMBEDDING]: 'Embeddings',
  [ObjectKind.EXPORT]: 'Exports',
  [ObjectKind.LABEL_EXAMPLE]: 'Label examples',
  [ObjectKind.BACKUP]: 'Backups',
  [ObjectKind.MODEL]: 'Models',
  [ObjectKind.UPLOAD]: 'Unfinished uploads',
  [ObjectKind.OTHER]: 'Other',
}

const muted = 'text-zinc-500 dark:text-zinc-400'
const bytes = (n: bigint) => formatBytes(Number(n))

function Categories({ categories }: { categories: ObjectCategory[] }) {
  const total = categories.reduce((sum, c) => sum + Number(c.bytes), 0)
  const shown = categories.filter((c) => c.objects > 0n || c.missingObjects > 0n)
  return (
    <Table className="mt-4 [--gutter:--spacing(6)] lg:[--gutter:--spacing(10)]">
      <TableHead>
        <TableRow>
          <TableHeader>Kind</TableHeader>
          <TableHeader className="text-right">Objects</TableHeader>
          <TableHeader className="text-right">Size</TableHeader>
          <TableHeader className="w-1/3">
            <span className="sr-only">Share</span>
          </TableHeader>
          <TableHeader className="text-right">Orphans</TableHeader>
          <TableHeader className="text-right">Missing</TableHeader>
        </TableRow>
      </TableHead>
      <TableBody>
        {shown.map((c) => (
          <TableRow key={c.kind}>
            <TableCell className="font-medium">{kindNames[c.kind]}</TableCell>
            <TableCell className="text-right tabular-nums">{formatNumber(c.objects)}</TableCell>
            <TableCell className="text-right tabular-nums">{bytes(c.bytes)}</TableCell>
            <TableCell>
              <Meter
                label={`${kindNames[c.kind]} share`}
                value={total > 0 ? Number(c.bytes) / total : 0}
              />
            </TableCell>
            <TableCell className={clsx('text-right tabular-nums', c.orphanObjects === 0n && muted)}>
              {c.orphanObjects > 0n
                ? `${formatNumber(c.orphanObjects)} · ${bytes(c.orphanBytes)}`
                : '—'}
            </TableCell>
            <TableCell
              className={clsx(
                'text-right tabular-nums',
                c.missingObjects > 0n ? 'text-amber-700 dark:text-amber-400' : muted,
              )}
            >
              {c.missingObjects > 0n ? formatNumber(c.missingObjects) : '—'}
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  )
}

function Missing({ categories }: { categories: ObjectCategory[] }) {
  const missing = categories.filter((c) => c.missingObjects > 0n)
  const total = missing.reduce((sum, c) => sum + c.missingObjects, 0n)
  if (total === 0n) return null
  return (
    <Callout type="warning" title={`${plural(total, 'row')} without an object`} className="mt-6">
      <details>
        <summary className="cursor-pointer">Examples</summary>
        <ul className="mt-2 space-y-1 font-mono text-xs/5">
          {missing.flatMap((c) =>
            c.missingSamples.map((k) => (
              <li key={k} className="truncate">
                {k}
              </li>
            )),
          )}
        </ul>
      </details>
    </Callout>
  )
}

function Cleanup({ scan, orphans }: { scan: StorageScan; orphans: Orphan[] }) {
  const [confirming, setConfirming] = useState(false)
  const objects = scan.categories.reduce((sum, c) => sum + c.orphanObjects, 0n)
  const size = scan.categories.reduce((sum, c) => sum + Number(c.orphanBytes), 0)
  const clean = useMutation(StorageService.method.cleanUpStorage, {
    onSuccess: async () => {
      await invalidateService(StorageService)
      setConfirming(false)
    },
  })
  const cleanup = scan.cleanup
  if (objects === 0n && !cleanup) return null

  return (
    <section className="mt-12">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <div>
          <Subheading>Orphans</Subheading>
          <p className={clsx('text-sm/6', muted)}>
            {plural(objects, 'object')} · {formatBytes(size)}
          </p>
        </div>
        {cleanup ? (
          <div className="flex items-center gap-3 text-sm/6">
            <JobStatusBadge status={cleanup.status} />
            {cleanup.status === JobStatus.DONE && (
              <span className={muted}>
                Deleted {plural(cleanup.deletedObjects, 'object')} · {bytes(cleanup.deletedBytes)}
                {cleanup.skippedObjects > 0n && `, kept ${formatNumber(cleanup.skippedObjects)}`}
              </span>
            )}
            {cleanup.status === JobStatus.FAILED && (
              <span className="text-red-700 dark:text-red-400">{cleanup.error}</span>
            )}
            {cleanup.status === JobStatus.FAILED && (
              <Button outline onClick={() => setConfirming(true)}>
                Retry
              </Button>
            )}
          </div>
        ) : (
          <Button outline onClick={() => setConfirming(true)}>
            Clean up
          </Button>
        )}
      </div>
      {orphans.length > 0 && (
        <Table dense className="mt-4 [--gutter:--spacing(6)] lg:[--gutter:--spacing(10)]">
          <TableHead>
            <TableRow>
              <TableHeader>Key</TableHeader>
              <TableHeader>Kind</TableHeader>
              <TableHeader className="text-right">Size</TableHeader>
              <TableHeader className="text-right">Modified</TableHeader>
            </TableRow>
          </TableHead>
          <TableBody>
            {orphans.map((o) => (
              <TableRow key={o.key + String(o.modifiedAt?.seconds)}>
                <TableCell className="max-w-md truncate font-mono text-xs">{o.key}</TableCell>
                <TableCell className={muted}>{kindNames[o.kind]}</TableCell>
                <TableCell className="text-right tabular-nums">{bytes(o.bytes)}</TableCell>
                <TableCell className={clsx('text-right', muted)}>
                  {o.modifiedAt && formatRelative(timestampDate(o.modifiedAt))}
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}
      <ConfirmAlert
        open={confirming}
        onClose={() => setConfirming(false)}
        onConfirm={() => clean.mutate({ scanId: scan.id })}
        title={`Delete ${plural(objects, 'orphan')}?`}
        description={`${formatBytes(size)}. Anything that gained a row or changed since the scan is kept.`}
        confirmLabel="Delete"
        busy={clean.isPending}
        error={clean.error}
      />
    </section>
  )
}

type SortKey = 'name' | 'frames' | 'embeddings' | 'source' | 'orphans' | 'total'

const total = (v: VideoUsage) =>
  Number(v.frameBytes + v.embeddingBytes + v.sourceBytes + v.orphanBytes)

const sortValue: Record<SortKey, (v: VideoUsage) => number | string> = {
  name: (v) => v.name || `~${v.videoId}`,
  frames: (v) => Number(v.frameBytes),
  embeddings: (v) => Number(v.embeddingBytes),
  source: (v) => Number(v.sourceBytes),
  orphans: (v) => Number(v.orphanBytes),
  total,
}

function SortHeader({
  label,
  column,
  sort,
  onSort,
  className,
}: {
  label: string
  column: SortKey
  sort: { key: SortKey; desc: boolean }
  onSort: (key: SortKey) => void
  className?: string
}) {
  const active = sort.key === column
  const Icon = active && !sort.desc ? ChevronUpIcon : ChevronDownIcon
  const icon = <Icon className={clsx('size-4', !active && 'invisible')} />
  // On right-aligned columns the icon goes first so the label lines up with
  // the numbers.
  const right = className?.includes('text-right')
  return (
    <TableHeader
      className={className}
      aria-sort={active ? (sort.desc ? 'descending' : 'ascending') : undefined}
    >
      <button
        type="button"
        onClick={() => onSort(column)}
        className="inline-flex items-center gap-1 hover:text-zinc-950 dark:hover:text-white"
      >
        {right && icon}
        {label}
        {!right && icon}
      </button>
    </TableHeader>
  )
}

function Videos({ videos }: { videos: VideoUsage[] }) {
  const [sort, setSort] = useState<{ key: SortKey; desc: boolean }>({ key: 'total', desc: true })
  const onSort = (key: SortKey) =>
    setSort((s) => (s.key === key ? { key, desc: !s.desc } : { key, desc: key !== 'name' }))
  const sorted = [...videos].sort((a, b) => {
    const x = sortValue[sort.key](a)
    const y = sortValue[sort.key](b)
    const c = typeof x === 'string' ? x.localeCompare(String(y)) : x - Number(y)
    return sort.desc ? -c : c
  })
  if (videos.length === 0) return null
  const props = { sort, onSort }

  return (
    <section className="mt-12">
      <Subheading>Videos</Subheading>
      <Table className="mt-4 [--gutter:--spacing(6)] lg:[--gutter:--spacing(10)]">
        <TableHead>
          <TableRow>
            <SortHeader label="Video" column="name" {...props} />
            <SortHeader label="Frames" column="frames" className="text-right" {...props} />
            <SortHeader label="Embeddings" column="embeddings" className="text-right" {...props} />
            <SortHeader label="Source" column="source" className="text-right" {...props} />
            <SortHeader label="Orphans" column="orphans" className="text-right" {...props} />
            <SortHeader label="Total" column="total" className="text-right" {...props} />
          </TableRow>
        </TableHead>
        <TableBody>
          {sorted.map((v) => (
            <TableRow key={String(v.videoId)} href={v.name ? `/videos/${v.videoId}` : undefined}>
              <TableCell className="max-w-xs truncate font-medium">
                {v.name || <span className={muted}>Deleted video {String(v.videoId)}</span>}
              </TableCell>
              <TableCell className="text-right tabular-nums">
                {bytes(v.frameBytes)}
                <span className={clsx('ml-2', muted)}>{formatNumber(v.frames)}</span>
              </TableCell>
              <TableCell className="text-right tabular-nums">
                {bytes(v.embeddingBytes)}
                <span className={clsx('ml-2', muted)}>{formatNumber(v.embeddings)}</span>
              </TableCell>
              <TableCell className="text-right tabular-nums">{bytes(v.sourceBytes)}</TableCell>
              <TableCell className={clsx('text-right tabular-nums', v.orphanBytes === 0n && muted)}>
                {v.orphanBytes > 0n ? bytes(v.orphanBytes) : '—'}
              </TableCell>
              <TableCell className="text-right font-medium tabular-nums">
                {formatBytes(total(v))}
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </section>
  )
}

function busy(scan: StorageScan | undefined) {
  return isActive(scan?.status) || isActive(scan?.cleanup?.status)
}

export function StoragePage() {
  const { data, isPending, error } = useQuery(
    StorageService.method.getStorageReport,
    {},
    { refetchInterval: (q) => (busy(q.state.data?.scan) ? 2000 : false) },
  )
  const scanNow = useMutation(StorageService.method.scanStorage, {
    onSuccess: () => invalidateService(StorageService),
    onError: (err) => flash.error('Could not start a scan', err),
  })
  const latest = data?.scan
  const shown = latest?.status === JobStatus.DONE ? latest : data?.previous
  const scanning = isActive(latest?.status)
  const objects = shown?.categories.reduce((sum, c) => sum + c.objects, 0n) ?? 0n
  const size = shown?.categories.reduce((sum, c) => sum + Number(c.bytes), 0) ?? 0

  return (
    <PageContentBlock title="Storage · Admin · Krill">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <div>
          <Heading>Storage</Heading>
          {shown?.finishedAt && (
            <p className={clsx('text-sm/6', muted)}>
              {formatBytes(size)} in {plural(objects, 'object')} · scanned{' '}
              {formatRelative(timestampDate(shown.finishedAt))}
            </p>
          )}
        </div>
        <Button
          outline
          onClick={() => scanNow.mutate({})}
          disabled={scanNow.isPending || busy(latest)}
        >
          <ArrowPathIcon data-slot="icon" className={scanning ? 'animate-spin' : ''} />
          {scanning ? 'Scanning' : 'Scan now'}
        </Button>
      </div>

      {isPending ? (
        <div className="flex justify-center py-16">
          <LoadingSpinner />
        </div>
      ) : error ? (
        <QueryError title="Could not load storage" error={error} className="mt-8" />
      ) : (
        <>
          {latest?.status === JobStatus.FAILED && (
            <Callout title="Scan failed" className="mt-8">
              {latest.error}
            </Callout>
          )}
          {!shown ? (
            !scanning && (
              <div className="mt-10">
                <EmptyState icon={ArchiveBoxIcon} title="No scan yet" description="" />
              </div>
            )
          ) : (
            <>
              <Categories categories={shown.categories} />
              <Missing categories={shown.categories} />
              <Cleanup scan={shown} orphans={data.orphans} />
              <Videos videos={shown.videos} />
            </>
          )}
        </>
      )}
    </PageContentBlock>
  )
}
