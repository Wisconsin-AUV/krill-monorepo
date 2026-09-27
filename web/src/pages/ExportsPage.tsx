import { useMutation, useQuery } from '@connectrpc/connect-query'
import { timestampDate } from '@bufbuild/protobuf/wkt'
import { ArrowDownTrayIcon, PlusIcon, TrashIcon } from '@heroicons/react/20/solid'
import { ArchiveBoxIcon } from '@heroicons/react/24/outline'
import { useState } from 'react'
import { ConfirmAlert } from '@/components/ConfirmAlert'
import { EmptyState } from '@/components/EmptyState'
import { ExportStatsView } from '@/components/ExportStatsView'
import { Meter } from '@/components/Meter'
import { NewExportDialog } from '@/components/NewExportDialog'
import { Badge, type BadgeColor } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import {
  Dialog,
  DialogActions,
  DialogBody,
  DialogDescription,
  DialogTitle,
} from '@/components/ui/Dialog'
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
import { Text } from '@/components/ui/Text'
import { ExportService, ExportStatus, type Export } from '@/gen/krill/v1/export_pb'
import { errorMessage } from '@/lib/errors'
import { flash } from '@/lib/flash'
import { formatBytes, formatNumber, formatRelative } from '@/lib/format'
import { invalidateService } from '@/lib/queryClient'

const statusMeta: Record<ExportStatus, { label: string; color: BadgeColor }> = {
  [ExportStatus.UNSPECIFIED]: { label: 'Unknown', color: 'zinc' },
  [ExportStatus.QUEUED]: { label: 'Queued', color: 'zinc' },
  [ExportStatus.RUNNING]: { label: 'Writing', color: 'sky' },
  [ExportStatus.READY]: { label: 'Ready', color: 'green' },
  [ExportStatus.FAILED]: { label: 'Failed', color: 'red' },
}

const active = (e: Export) => e.status === ExportStatus.QUEUED || e.status === ExportStatus.RUNNING

function optionSummary(e: Export): string {
  const o = e.options
  if (!o) return ''
  const parts = [`${Math.round(o.valFraction * 100)}% val`]
  if (o.stride > 1) parts.push(`stride ${o.stride}`)
  if (o.dedup) parts.push(`dedup ≤${o.dedupDistance}`)
  if (o.videoIds.length > 0) parts.push(`${o.videoIds.length} videos`)
  return parts.join(' · ')
}

function ExportDetails({ exp, onClose }: { exp: Export | null; onClose: () => void }) {
  return (
    <Dialog open={exp !== null} onClose={onClose} size="3xl">
      {exp && (
        <>
          <DialogTitle>{exp.name}</DialogTitle>
          <DialogDescription>{optionSummary(exp)}</DialogDescription>
          <DialogBody>
            {exp.error && (
              <p className="mb-6 rounded-lg bg-red-50 px-3 py-2 text-sm/6 text-red-700 dark:bg-red-500/10 dark:text-red-400">
                {exp.error}
              </p>
            )}
            {exp.stats && <ExportStatsView stats={exp.stats} />}
          </DialogBody>
          <DialogActions>
            <Button plain onClick={onClose}>
              Close
            </Button>
            {exp.downloadUrl && (
              <Button color="sky" to={exp.downloadUrl}>
                <ArrowDownTrayIcon data-slot="icon" />
                Download
              </Button>
            )}
          </DialogActions>
        </>
      )}
    </Dialog>
  )
}

export function ExportsPage() {
  const [creating, setCreating] = useState(false)
  const [viewing, setViewing] = useState<Export | null>(null)
  const [deleting, setDeleting] = useState<Export | null>(null)
  const { data, isPending, error } = useQuery(
    ExportService.method.listExports,
    {},
    { refetchInterval: (q) => (q.state.data?.exports.some(active) ? 1500 : false) },
  )
  const remove = useMutation(ExportService.method.deleteExport, {
    onSuccess: async () => {
      await invalidateService(ExportService)
      setDeleting(null)
    },
    onError: (err) => flash.error('Could not delete export', err),
  })
  const exports = data?.exports ?? []

  return (
    <PageContentBlock title="Exports · Krill">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <Heading>Exports</Heading>
          <Text className="mt-1">
            Versioned YOLO datasets built from frames marked labeled or empty.
          </Text>
        </div>
        <Button color="sky" onClick={() => setCreating(true)}>
          <PlusIcon data-slot="icon" />
          New export
        </Button>
      </div>

      {isPending ? (
        <div className="flex justify-center py-16">
          <LoadingSpinner />
        </div>
      ) : error ? (
        <Text className="mt-8 text-red-600 dark:text-red-400">
          Could not load exports: {errorMessage(error)}
        </Text>
      ) : exports.length === 0 ? (
        <div className="mt-10">
          <EmptyState
            icon={ArchiveBoxIcon}
            title="No exports yet"
            description="Label some frames, then export a dataset to train YOLO on."
          >
            <Button color="sky" onClick={() => setCreating(true)}>
              <PlusIcon data-slot="icon" />
              New export
            </Button>
          </EmptyState>
        </div>
      ) : (
        <Table className="mt-8 [--gutter:--spacing(6)] lg:[--gutter:--spacing(10)]">
          <TableHead>
            <TableRow>
              <TableHeader>Export</TableHeader>
              <TableHeader>Status</TableHeader>
              <TableHeader className="text-right">Train</TableHeader>
              <TableHeader className="text-right">Val</TableHeader>
              <TableHeader className="text-right">Boxes</TableHeader>
              <TableHeader className="text-right">Size</TableHeader>
              <TableHeader className="text-right">Created</TableHeader>
              <TableHeader className="relative w-0">
                <span className="sr-only">Actions</span>
              </TableHeader>
            </TableRow>
          </TableHead>
          <TableBody>
            {exports.map((e) => {
              const s = e.stats
              return (
                <TableRow key={String(e.id)}>
                  <TableCell>
                    <button type="button" onClick={() => setViewing(e)} className="text-left">
                      <div className="font-medium text-zinc-950 hover:underline dark:text-white">
                        {e.name}
                      </div>
                      <div className="text-zinc-500 dark:text-zinc-400">{optionSummary(e)}</div>
                    </button>
                  </TableCell>
                  <TableCell>
                    <Badge color={statusMeta[e.status].color}>{statusMeta[e.status].label}</Badge>
                    {e.status === ExportStatus.RUNNING && (
                      <Meter value={e.progress} label={`Writing ${e.name}`} className="mt-2 w-24" />
                    )}
                  </TableCell>
                  <TableCell className="text-right tabular-nums">
                    {formatNumber(s?.trainImages ?? 0)}
                  </TableCell>
                  <TableCell className="text-right tabular-nums">
                    {formatNumber(s?.valImages ?? 0)}
                  </TableCell>
                  <TableCell className="text-right tabular-nums">
                    {formatNumber((s?.trainBoxes ?? 0) + (s?.valBoxes ?? 0))}
                  </TableCell>
                  <TableCell className="text-right text-zinc-500 tabular-nums dark:text-zinc-400">
                    {e.sizeBytes > 0n ? formatBytes(Number(e.sizeBytes)) : '—'}
                  </TableCell>
                  <TableCell className="text-right text-zinc-500 dark:text-zinc-400">
                    {e.createdAt && formatRelative(timestampDate(e.createdAt))}
                  </TableCell>
                  <TableCell>
                    <div className="flex justify-end gap-1">
                      {e.downloadUrl && (
                        <Button outline to={e.downloadUrl}>
                          <ArrowDownTrayIcon data-slot="icon" />
                          Download
                        </Button>
                      )}
                      <Button
                        plain
                        aria-label={`Delete ${e.name}`}
                        disabled={active(e)}
                        onClick={() => setDeleting(e)}
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
      )}

      <NewExportDialog open={creating} onClose={() => setCreating(false)} />
      <ExportDetails exp={viewing} onClose={() => setViewing(null)} />
      <ConfirmAlert
        open={deleting !== null}
        onClose={() => setDeleting(null)}
        onConfirm={() => deleting && remove.mutate({ id: deleting.id })}
        busy={remove.isPending}
        title={`Delete ${deleting?.name ?? ''}?`}
        description="The zip is removed from storage. Labels are not affected."
        confirmLabel="Delete export"
      />
    </PageContentBlock>
  )
}
