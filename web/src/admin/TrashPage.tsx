import { timestampDate } from '@bufbuild/protobuf/wkt'
import { useMutation, useQuery } from '@connectrpc/connect-query'
import { ArrowUturnLeftIcon, TrashIcon } from '@heroicons/react/20/solid'
import { TrashIcon as TrashOutlineIcon } from '@heroicons/react/24/outline'
import { useState } from 'react'
import { ConfirmAlert } from '@/components/ConfirmAlert'
import { EmptyState } from '@/components/EmptyState'
import { Thumbnail } from '@/components/Thumbnail'
import { Avatar } from '@/components/ui/Avatar'
import { Button } from '@/components/ui/Button'
import { QueryError } from '@/components/ui/Callout'
import { Checkbox } from '@/components/ui/Checkbox'
import { Heading } from '@/components/ui/Heading'
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
import { ClipService } from '@/gen/krill/v1/clip_pb'
import { ContentService, TrashKind, type TrashItem } from '@/gen/krill/v1/content_pb'
import { QueueService } from '@/gen/krill/v1/queue_pb'
import { VideoService } from '@/gen/krill/v1/video_pb'
import { flash } from '@/lib/flash'
import { formatBytes, formatNumber, formatRelative, plural } from '@/lib/format'
import { invalidateService } from '@/lib/queryClient'
import { useSelection } from './selection'

const key = (i: TrashItem) => `${i.kind === TrashKind.VIDEO ? 'v' : 'c'}${i.id}`

function itemName(i: TrashItem) {
  return i.kind === TrashKind.VIDEO ? i.videoName : `${i.videoName} · Clip ${i.clipIndex + 1}`
}

function split(items: TrashItem[]) {
  return {
    videoIds: items.filter((i) => i.kind === TrashKind.VIDEO).map((i) => i.id),
    clipIds: items.filter((i) => i.kind === TrashKind.CLIP).map((i) => i.id),
  }
}

function count(items: TrashItem[]) {
  const { videoIds, clipIds } = split(items)
  return [
    videoIds.length > 0 && plural(videoIds.length, 'video'),
    clipIds.length > 0 && plural(clipIds.length, 'clip'),
  ]
    .filter(Boolean)
    .join(', ')
}

function refresh() {
  return Promise.all([
    invalidateService(ContentService),
    invalidateService(VideoService),
    invalidateService(ClipService),
    invalidateService(QueueService),
  ])
}

export function TrashPage() {
  const { data, isPending, error } = useQuery(ContentService.method.listTrash, {})
  const items = data?.items ?? []
  const selection = useSelection(items.map(key))
  const picked = items.filter((i) => selection.has(key(i)))
  // Items to delete forever, or 'all' to empty the trash.
  const [purging, setPurging] = useState<TrashItem[] | 'all' | null>(null)

  const restore = useMutation(ContentService.method.restoreTrash, {
    onSuccess: async (res) => {
      await refresh()
      selection.clear()
      flash.success(
        `Restored ${[res.videos > 0 && plural(res.videos, 'video'), res.clips > 0 && plural(res.clips, 'clip')].filter(Boolean).join(', ')}`,
      )
    },
    onError: (err) => flash.error('Could not restore', err),
  })
  const purge = useMutation(ContentService.method.purgeTrash, {
    onSuccess: async () => {
      await refresh()
      selection.clear()
      setPurging(null)
    },
  })
  const empty = useMutation(ContentService.method.emptyTrash, {
    onSuccess: async () => {
      await refresh()
      selection.clear()
      setPurging(null)
    },
  })
  const purgeTargets = purging === 'all' ? items : (purging ?? [])
  const purgeBoxes = purgeTargets.reduce((n, i) => n + i.boxCount, 0)

  return (
    <PageContentBlock title="Trash · Admin · Krill">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <Heading>Trash</Heading>
        <div className="flex items-center gap-3">
          {data && data.retentionDays > 0 && (
            <span className="text-sm/6 text-zinc-500 dark:text-zinc-400">
              Kept {plural(data.retentionDays, 'day')}
            </span>
          )}
          <Button outline disabled={items.length === 0} onClick={() => setPurging('all')}>
            <TrashIcon data-slot="icon" />
            Empty trash
          </Button>
        </div>
      </div>

      {isPending ? (
        <div className="flex justify-center py-16">
          <LoadingSpinner />
        </div>
      ) : error ? (
        <QueryError title="Could not load the trash" error={error} className="mt-8" />
      ) : items.length === 0 ? (
        <div className="mt-10">
          <EmptyState icon={TrashOutlineIcon} title="Trash is empty" description="" />
        </div>
      ) : (
        <>
          <div className="mt-8 flex h-9 items-center gap-3">
            {picked.length > 0 && (
              <>
                <span className="text-sm/6 font-medium text-zinc-950 dark:text-white">
                  {formatNumber(picked.length)} selected
                </span>
                <Button
                  outline
                  disabled={restore.isPending}
                  onClick={() => restore.mutate(split(picked))}
                >
                  <ArrowUturnLeftIcon data-slot="icon" />
                  Restore
                </Button>
                <Button outline onClick={() => setPurging(picked)}>
                  <TrashIcon data-slot="icon" />
                  Delete forever
                </Button>
              </>
            )}
          </div>
          <Table className="mt-2 [--gutter:--spacing(6)] lg:[--gutter:--spacing(10)]">
            <TableHead>
              <TableRow>
                <TableHeader className="w-0">
                  <Checkbox
                    aria-label="Select all"
                    checked={selection.all}
                    indeterminate={selection.some}
                    onChange={selection.toggleAll}
                  />
                </TableHeader>
                <TableHeader>Item</TableHeader>
                <TableHeader>Deleted</TableHeader>
                <TableHeader className="text-right">Boxes</TableHeader>
                <TableHeader className="text-right">Size</TableHeader>
                {data.retentionDays > 0 && <TableHeader>Purge</TableHeader>}
                <TableHeader className="relative w-0">
                  <span className="sr-only">Actions</span>
                </TableHeader>
              </TableRow>
            </TableHead>
            <TableBody>
              {items.map((i) => {
                const deletedAt = i.deletedAt ? timestampDate(i.deletedAt) : null
                const purgeAt = i.purgeAt ? timestampDate(i.purgeAt) : null
                return (
                  <TableRow key={key(i)}>
                    <TableCell>
                      <Checkbox
                        aria-label={`Select ${itemName(i)}`}
                        checked={selection.has(key(i))}
                        onChange={(on) => selection.toggle(key(i), on)}
                      />
                    </TableCell>
                    <TableCell>
                      <div className="flex items-center gap-3">
                        <Thumbnail src={i.thumbnailUrl} className="w-20 shrink-0" />
                        <div className="min-w-0">
                          <div className="truncate font-medium text-zinc-950 dark:text-white">
                            {itemName(i)}
                          </div>
                          <div className="text-zinc-500 dark:text-zinc-400">
                            {i.kind === TrashKind.VIDEO
                              ? `${plural(i.clipCount, 'clip')} · ${plural(i.frameCount, 'frame')}`
                              : plural(i.frameCount, 'frame')}
                          </div>
                        </div>
                      </div>
                    </TableCell>
                    <TableCell>
                      <div className="flex items-center gap-2">
                        {i.deletedBy && <Avatar seed={i.deletedBy.id} className="size-6" />}
                        <div>
                          <div className="text-zinc-950 dark:text-white">
                            {i.deletedBy?.name ?? 'Unknown'}
                          </div>
                          <div
                            className="text-zinc-500 dark:text-zinc-400"
                            title={deletedAt?.toLocaleString()}
                          >
                            {deletedAt ? formatRelative(deletedAt) : ''}
                          </div>
                        </div>
                      </div>
                    </TableCell>
                    <TableCell
                      className="text-right tabular-nums"
                      title={`${plural(i.trackCount, 'track')} · ${plural(i.labeledFrameCount, 'labeled frame')}`}
                    >
                      {formatNumber(i.boxCount)}
                    </TableCell>
                    <TableCell className="text-right whitespace-nowrap tabular-nums">
                      {i.sizeBytes < 0n ? '—' : formatBytes(Number(i.sizeBytes))}
                    </TableCell>
                    {data.retentionDays > 0 && (
                      <TableCell
                        className="whitespace-nowrap text-zinc-500 dark:text-zinc-400"
                        title={purgeAt?.toLocaleString()}
                      >
                        {purgeAt ? formatRelative(purgeAt) : ''}
                      </TableCell>
                    )}
                    <TableCell>
                      <div className="flex justify-end gap-1">
                        <Button
                          plain
                          disabled={restore.isPending}
                          onClick={() => restore.mutate(split([i]))}
                        >
                          Restore
                        </Button>
                        <Button
                          plain
                          aria-label={`Delete ${itemName(i)} forever`}
                          title="Delete forever"
                          onClick={() => setPurging([i])}
                        >
                          <TrashIcon data-slot="icon" />
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                )
              })}
            </TableBody>
          </Table>
        </>
      )}

      <ConfirmAlert
        open={purging !== null}
        onClose={() => {
          setPurging(null)
          purge.reset()
          empty.reset()
        }}
        onConfirm={() => (purging === 'all' ? empty.mutate({}) : purge.mutate(split(purgeTargets)))}
        busy={purge.isPending || empty.isPending}
        error={purge.error ?? empty.error}
        title={
          purging === 'all'
            ? 'Empty trash?'
            : purgeTargets.length === 1
              ? `Delete ${itemName(purgeTargets[0])} forever?`
              : `Delete ${count(purgeTargets)} forever?`
        }
        description={`${count(purgeTargets)} · ${plural(purgeBoxes, 'box', 'boxes')}`}
        confirmLabel="Delete forever"
      />
    </PageContentBlock>
  )
}
