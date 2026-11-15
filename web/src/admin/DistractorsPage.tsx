import { useMutation, useQuery } from '@connectrpc/connect-query'
import { ArrowUpTrayIcon, PhotoIcon, TrashIcon } from '@heroicons/react/20/solid'
import { clsx } from 'clsx'
import { useState } from 'react'
import { DistractorUploadDialog } from '@/admin/DistractorUploadDialog'
import { useSelection } from '@/admin/selection'
import { ConfirmAlert } from '@/components/ConfirmAlert'
import { EmptyState } from '@/components/EmptyState'
import { Thumbnail } from '@/components/Thumbnail'
import { Button } from '@/components/ui/Button'
import { QueryError } from '@/components/ui/Callout'
import { Checkbox, CheckboxField } from '@/components/ui/Checkbox'
import { Dialog, DialogBody, DialogTitle } from '@/components/ui/Dialog'
import { Label } from '@/components/ui/Fieldset'
import { Heading } from '@/components/ui/Heading'
import LoadingSpinner from '@/components/ui/LoadingSpinner'
import PageContentBlock from '@/components/ui/PageContentBlock'
import { DistractorService, DistractorStatus, type Distractor } from '@/gen/krill/v1/distractor_pb'
import { flash } from '@/lib/flash'
import { formatBytes, formatNumber, plural } from '@/lib/format'
import { invalidateService } from '@/lib/queryClient'

function processing(d: Distractor) {
  return d.status === DistractorStatus.QUEUED || d.status === DistractorStatus.PROCESSING
}

export function DistractorsPage() {
  const { data, isPending, error } = useQuery(
    DistractorService.method.listDistractors,
    {},
    { refetchInterval: (q) => (q.state.data?.distractors.some(processing) ? 2000 : false) },
  )
  const distractors = data?.distractors ?? []
  const selection = useSelection(distractors.map((d) => String(d.id)))
  const [uploading, setUploading] = useState(false)
  const [deleting, setDeleting] = useState(false)
  const [viewing, setViewing] = useState<Distractor | null>(null)
  const remove = useMutation(DistractorService.method.deleteDistractors, {
    onSuccess: async () => {
      await invalidateService(DistractorService)
      selection.clear()
      setDeleting(false)
    },
  })
  const retry = useMutation(DistractorService.method.processDistractor, {
    onSuccess: () => invalidateService(DistractorService),
    onError: (err) => flash.error('Could not retry', err),
  })
  const bytes = distractors.reduce((sum, d) => sum + Number(d.sizeBytes), 0)

  return (
    <PageContentBlock title="Distractors · Admin · Krill">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <Heading>Distractors</Heading>
        <div className="flex items-center gap-3">
          {data && distractors.length > 0 && (
            <span className="text-sm/6 text-zinc-500 tabular-nums dark:text-zinc-400">
              {plural(distractors.length, 'image')} · {formatBytes(bytes)}
            </span>
          )}
          <Button color="sky" onClick={() => setUploading(true)}>
            <ArrowUpTrayIcon data-slot="icon" />
            Upload
          </Button>
        </div>
      </div>

      {isPending ? (
        <div className="flex justify-center py-16">
          <LoadingSpinner />
        </div>
      ) : error ? (
        <QueryError title="Could not load distractors" error={error} className="mt-8" />
      ) : distractors.length === 0 ? (
        <div className="mt-8">
          <EmptyState
            icon={PhotoIcon}
            title="No distractors"
            description="Scenes with none of the labeled objects."
          />
        </div>
      ) : (
        <>
          <div className="mt-8 flex h-9 items-center gap-4">
            <CheckboxField>
              <Checkbox
                checked={selection.all}
                indeterminate={selection.some}
                onChange={selection.toggleAll}
              />
              <Label>
                {selection.selected.length > 0
                  ? `${formatNumber(selection.selected.length)} selected`
                  : 'Select all'}
              </Label>
            </CheckboxField>
            {selection.selected.length > 0 && (
              <Button outline onClick={() => setDeleting(true)}>
                <TrashIcon data-slot="icon" />
                Delete
              </Button>
            )}
          </div>
          <ul className="mt-4 grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5">
            {distractors.map((d) => (
              <Tile
                key={String(d.id)}
                distractor={d}
                selected={selection.has(String(d.id))}
                selecting={selection.selected.length > 0}
                onSelect={(on) => selection.toggle(String(d.id), on)}
                onView={() => setViewing(d)}
                onRetry={() => retry.mutate({ id: d.id })}
                retrying={retry.isPending && retry.variables?.id === d.id}
              />
            ))}
          </ul>
        </>
      )}

      <DistractorUploadDialog open={uploading} onClose={() => setUploading(false)} />

      <Dialog open={viewing !== null} onClose={() => setViewing(null)} size="5xl">
        <DialogTitle className="truncate">{viewing?.filename || 'Distractor'}</DialogTitle>
        <DialogBody>
          {viewing && (
            <img
              src={viewing.url}
              alt={viewing.filename}
              width={viewing.width}
              height={viewing.height}
              className="mx-auto max-h-[70vh] w-auto rounded-md object-contain"
            />
          )}
        </DialogBody>
      </Dialog>

      <ConfirmAlert
        open={deleting}
        onClose={() => {
          setDeleting(false)
          remove.reset()
        }}
        onConfirm={() => remove.mutate({ ids: selection.selected.map((id) => BigInt(id)) })}
        busy={remove.isPending}
        error={remove.error}
        title={`Delete ${plural(selection.selected.length, 'distractor')}?`}
        description="The images are removed from storage."
        confirmLabel="Delete"
      />
    </PageContentBlock>
  )
}

function Tile({
  distractor: d,
  selected,
  selecting,
  onSelect,
  onView,
  onRetry,
  retrying,
}: {
  distractor: Distractor
  selected: boolean
  selecting: boolean
  onSelect: (on: boolean) => void
  onView: () => void
  onRetry: () => void
  retrying: boolean
}) {
  const ready = d.status === DistractorStatus.READY
  return (
    <li className="group relative min-w-0">
      {ready ? (
        <button
          type="button"
          onClick={onView}
          aria-label={`View ${d.filename}`}
          className="block w-full rounded-md focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-sky-500"
        >
          <Thumbnail src={d.thumbnailUrl} alt="" className="transition group-hover:opacity-90" />
        </button>
      ) : (
        <div className="relative">
          <Thumbnail />
          {d.status !== DistractorStatus.FAILED && (
            <div className="absolute inset-0 flex items-center justify-center">
              <LoadingSpinner />
            </div>
          )}
        </div>
      )}
      <div
        className={clsx(
          'absolute top-2 left-2',
          !selecting &&
            'opacity-0 group-hover:opacity-100 focus-within:opacity-100 pointer-coarse:opacity-100',
        )}
      >
        <Checkbox aria-label={`Select ${d.filename}`} checked={selected} onChange={onSelect} />
      </div>
      <p className="mt-1.5 truncate text-sm/5 text-zinc-950 dark:text-white" title={d.filename}>
        {d.filename}
      </p>
      {ready ? (
        <p className="text-xs/5 text-zinc-500 tabular-nums dark:text-zinc-400">
          {d.width}×{d.height} · {formatBytes(Number(d.sizeBytes))}
        </p>
      ) : d.status === DistractorStatus.FAILED ? (
        <div className="flex items-center gap-2 text-xs/5">
          <span className="min-w-0 truncate text-red-600 dark:text-red-400" title={d.error}>
            {d.error || 'Failed'}
          </span>
          <button
            type="button"
            onClick={onRetry}
            disabled={retrying}
            className="shrink-0 font-medium text-sky-600 hover:text-sky-500 disabled:opacity-50 dark:text-sky-400"
          >
            Retry
          </button>
        </div>
      ) : (
        <p className="text-xs/5 text-zinc-500 dark:text-zinc-400">
          {d.status === DistractorStatus.UPLOADING ? 'Uploading' : 'Processing'}
        </p>
      )}
    </li>
  )
}
