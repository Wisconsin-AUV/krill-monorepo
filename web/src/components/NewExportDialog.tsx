import { create } from '@bufbuild/protobuf'
import { useMutation, useQuery } from '@connectrpc/connect-query'
import { keepPreviousData } from '@tanstack/react-query'
import { useState } from 'react'
import { ExportStatsView } from '@/components/ExportStatsView'
import { Button } from '@/components/ui/Button'
import { FormError } from '@/components/ui/Callout'
import { Checkbox, CheckboxField } from '@/components/ui/Checkbox'
import { Dialog, DialogActions, DialogBody, DialogTitle } from '@/components/ui/Dialog'
import { Field, FieldGroup, Fieldset, Label, Legend } from '@/components/ui/Fieldset'
import { Input } from '@/components/ui/Input'
import LoadingSpinner from '@/components/ui/LoadingSpinner'
import { Select } from '@/components/ui/Select'
import { ExportOptionsSchema, ExportService, type ExportOptions } from '@/gen/krill/v1/export_pb'
import { VideoService, VideoStatus } from '@/gen/krill/v1/video_pb'
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

// Adds the admin panel's default to a list of choices when it isn't one.
function withValue<T extends { value: number }>(
  options: T[],
  value: number,
  make: (v: number) => T,
) {
  return options.some((o) => o.value === value)
    ? options
    : [...options, make(value)].sort((a, b) => a.value - b.value)
}

function NewExportForm({ defaults, onClose }: { defaults: ExportOptions; onClose: () => void }) {
  const [name, setName] = useState(`krill-${today()}`)
  const [valFraction, setValFraction] = useState(defaults.valFraction)
  const [stride, setStride] = useState(defaults.stride || 1)
  const [dedup, setDedup] = useState(defaults.dedup)
  const [dedupDistance, setDedupDistance] = useState(defaults.dedupDistance || 4)
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
  })

  const stats = preview.data?.stats
  const images = stats ? stats.trainImages + stats.valImages + stats.testImages : 0

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
      <DialogBody className="grid gap-10 lg:grid-cols-[minmax(0,5fr)_minmax(0,7fr)]">
        <FieldGroup>
          <Field>
            <Label>Name</Label>
            <Input value={name} onChange={(e) => setName(e.target.value)} maxLength={100} />
          </Field>
          <Field>
            <Label>Validation share</Label>
            <Select value={valFraction} onChange={(e) => setValFraction(Number(e.target.value))}>
              {[...new Set([...valOptions, defaults.valFraction])]
                .sort((a, b) => a - b)
                .map((v) => (
                  <option key={v} value={v}>
                    {Math.round(v * 100)}%
                  </option>
                ))}
            </Select>
          </Field>
          <Field>
            <Label>Frame stride</Label>
            <Select value={stride} onChange={(e) => setStride(Number(e.target.value))}>
              {withValue(strideOptions, defaults.stride || 1, (v) => ({
                value: v,
                label: `Every ${v}th frame`,
              })).map((o) => (
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
            </CheckboxField>
            {dedup && (
              <Select
                className="mt-3"
                aria-label="Duplicate threshold"
                value={dedupDistance}
                onChange={(e) => setDedupDistance(Number(e.target.value))}
              >
                {withValue(dedupOptions, defaults.dedupDistance || 4, (v) => ({
                  value: v,
                  label: `Within ${v} bits`,
                })).map((o) => (
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
          <FormError error={createExport.error} />
        </FieldGroup>

        <div className="min-w-0">
          <div className="mb-4 flex items-center justify-between">
            <h3 className="text-sm/6 font-semibold text-zinc-950 dark:text-white">Preview</h3>
            {preview.isFetching && <LoadingSpinner />}
          </div>
          {preview.error ? (
            <FormError error={preview.error} />
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
  const { data } = useQuery(ExportService.method.getExportDefaults, {}, { enabled: open })
  return (
    <Dialog open={open} onClose={onClose} size="5xl">
      {open &&
        (data?.options ? (
          <NewExportForm defaults={data.options} onClose={onClose} />
        ) : (
          <div className="flex justify-center py-16">
            <LoadingSpinner />
          </div>
        ))}
    </Dialog>
  )
}
