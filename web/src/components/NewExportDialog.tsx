import { create } from '@bufbuild/protobuf'
import { useMutation, useQuery } from '@connectrpc/connect-query'
import { keepPreviousData } from '@tanstack/react-query'
import { useState } from 'react'
import { ExportStatsView } from '@/components/ExportStatsView'
import { Button } from '@/components/ui/Button'
import { Checkbox, CheckboxField } from '@/components/ui/Checkbox'
import {
  Dialog,
  DialogActions,
  DialogBody,
  DialogDescription,
  DialogTitle,
} from '@/components/ui/Dialog'
import { Description, Field, FieldGroup, Fieldset, Label, Legend } from '@/components/ui/Fieldset'
import { Input } from '@/components/ui/Input'
import LoadingSpinner from '@/components/ui/LoadingSpinner'
import { Select } from '@/components/ui/Select'
import { ExportOptionsSchema, ExportService } from '@/gen/krill/v1/export_pb'
import { VideoService, VideoStatus } from '@/gen/krill/v1/video_pb'
import { errorMessage } from '@/lib/errors'
import { flash } from '@/lib/flash'
import { plural } from '@/lib/format'
import { invalidateService } from '@/lib/queryClient'
import { splitLabel } from '@/lib/video'

const valOptions = [0.1, 0.15, 0.2, 0.25, 0.3]
const strideOptions = [
  { value: 1, label: 'Every frame' },
  { value: 2, label: 'Every 2nd frame' },
  { value: 3, label: 'Every 3rd frame' },
  { value: 5, label: 'Every 5th frame' },
  { value: 10, label: 'Every 10th frame' },
]
const dedupOptions = [
  { value: 2, label: 'Strict (only near-identical)' },
  { value: 4, label: 'Normal' },
  { value: 8, label: 'Loose' },
]

function today() {
  return new Date().toISOString().slice(0, 10)
}

function NewExportForm({ onClose }: { onClose: () => void }) {
  const [name, setName] = useState(`krill-${today()}`)
  const [valFraction, setValFraction] = useState(0.2)
  const [stride, setStride] = useState(1)
  const [dedup, setDedup] = useState(true)
  const [dedupDistance, setDedupDistance] = useState(4)
  const [videoIds, setVideoIds] = useState<bigint[] | null>(null)

  const { data: videoData } = useQuery(VideoService.method.listVideos, {})
  const ready = (videoData?.videos ?? []).filter((v) => v.status === VideoStatus.READY)
  const selected = videoIds ?? ready.map((v) => v.id)

  const options = create(ExportOptionsSchema, {
    valFraction,
    stride,
    dedup,
    dedupDistance,
    videoIds: videoIds ?? [],
  })
  const preview = useQuery(
    ExportService.method.previewExport,
    { options },
    { placeholderData: keepPreviousData, enabled: videoIds === null || videoIds.length > 0 },
  )
  const createExport = useMutation(ExportService.method.createExport, {
    onSuccess: async () => {
      await invalidateService(ExportService)
      flash.success('Export queued', 'The download appears here when it is ready.')
      onClose()
    },
    onError: (err) => flash.error('Could not create export', err),
  })

  const stats = preview.data?.stats
  const images = stats ? stats.trainImages + stats.valImages : 0

  function toggleVideo(id: bigint, on: boolean) {
    const next = on ? [...selected, id] : selected.filter((v) => v !== id)
    setVideoIds(next.length === ready.length ? null : next)
  }

  return (
    <form
      onSubmit={(e) => {
        e.preventDefault()
        createExport.mutate({ name, options })
      }}
    >
      <DialogTitle>New export</DialogTitle>
      <DialogDescription>
        A YOLO dataset zip with images, labels, data.yaml, and a manifest. Train and val are split
        by video.
      </DialogDescription>
      <DialogBody className="grid gap-10 lg:grid-cols-[minmax(0,5fr)_minmax(0,7fr)]">
        <FieldGroup>
          <Field>
            <Label>Name</Label>
            <Input value={name} onChange={(e) => setName(e.target.value)} maxLength={100} />
          </Field>
          <Field>
            <Label>Validation share</Label>
            <Description>
              Videos pinned to a split keep it. The rest are balanced toward this share.
            </Description>
            <Select value={valFraction} onChange={(e) => setValFraction(Number(e.target.value))}>
              {valOptions.map((v) => (
                <option key={v} value={v}>
                  {Math.round(v * 100)}%
                </option>
              ))}
            </Select>
          </Field>
          <Field>
            <Label>Frame stride</Label>
            <Select value={stride} onChange={(e) => setStride(Number(e.target.value))}>
              {strideOptions.map((o) => (
                <option key={o.value} value={o.value}>
                  {o.label}
                </option>
              ))}
            </Select>
          </Field>
          <Fieldset>
            <CheckboxField>
              <Checkbox checked={dedup} onChange={setDedup} color="sky" />
              <Label>Drop near-duplicate frames</Label>
              <Description>
                Compares each frame's perceptual hash with the last one kept.
              </Description>
            </CheckboxField>
            {dedup && (
              <Select
                className="mt-3"
                aria-label="Duplicate threshold"
                value={dedupDistance}
                onChange={(e) => setDedupDistance(Number(e.target.value))}
              >
                {dedupOptions.map((o) => (
                  <option key={o.value} value={o.value}>
                    {o.label}
                  </option>
                ))}
              </Select>
            )}
          </Fieldset>
          <Fieldset>
            <Legend>Videos</Legend>
            <div className="mt-3 max-h-56 space-y-3 overflow-y-auto pr-1">
              {ready.map((v) => (
                <CheckboxField key={String(v.id)}>
                  <Checkbox
                    checked={selected.includes(v.id)}
                    onChange={(on) => toggleVideo(v.id, on)}
                    color="sky"
                  />
                  <Label className="flex justify-between gap-3">
                    <span className="truncate">{v.name}</span>
                    <span className="shrink-0 text-xs text-zinc-500 dark:text-zinc-400">
                      {splitLabel(v.split)}
                    </span>
                  </Label>
                </CheckboxField>
              ))}
            </div>
          </Fieldset>
        </FieldGroup>

        <div className="min-w-0">
          <div className="mb-4 flex items-center justify-between">
            <h3 className="text-sm/6 font-semibold text-zinc-950 dark:text-white">Preview</h3>
            {preview.isFetching && <LoadingSpinner />}
          </div>
          {preview.error ? (
            <p className="text-sm/6 text-red-600 dark:text-red-400">
              {errorMessage(preview.error)}
            </p>
          ) : stats ? (
            <ExportStatsView stats={stats} />
          ) : (
            <p className="text-sm/6 text-zinc-500 dark:text-zinc-400">Select at least one video.</p>
          )}
        </div>
      </DialogBody>
      <DialogActions>
        <Button plain onClick={onClose}>
          Cancel
        </Button>
        <Button
          type="submit"
          color="sky"
          disabled={createExport.isPending || images === 0 || selected.length === 0}
        >
          {images > 0 ? `Export ${plural(images, 'image')}` : 'Nothing to export'}
        </Button>
      </DialogActions>
    </form>
  )
}

export function NewExportDialog({ open, onClose }: { open: boolean; onClose: () => void }) {
  return (
    <Dialog open={open} onClose={onClose} size="5xl">
      {open && <NewExportForm onClose={onClose} />}
    </Dialog>
  )
}
