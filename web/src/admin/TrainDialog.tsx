import { create } from '@bufbuild/protobuf'
import { useMutation } from '@connectrpc/connect-query'
import { ChevronRightIcon } from '@heroicons/react/20/solid'
import { clsx } from 'clsx'
import { useState } from 'react'
import { useNavigate } from 'react-router'
import { Button } from '@/components/ui/Button'
import { FormError } from '@/components/ui/Callout'
import { Checkbox, CheckboxField } from '@/components/ui/Checkbox'
import { Dialog, DialogActions, DialogBody, DialogTitle } from '@/components/ui/Dialog'
import { Field, FieldGroup, Fieldset, Label, Legend } from '@/components/ui/Fieldset'
import { Input } from '@/components/ui/Input'
import { Select } from '@/components/ui/Select'
import {
  AugmentationSchema,
  ExportOptionsSchema,
  type Augmentation,
} from '@/gen/krill/v1/export_pb'
import {
  ModelSize,
  PipelineService,
  TrainConfigSchema,
  TrainingStatus,
  type TrainConfig,
  type TrainingRun,
} from '@/gen/krill/v1/pipeline_pb'
import { invalidateService } from '@/lib/queryClient'
import { float32, runName, sizeNames } from './pipeline'

function NumberField({
  label,
  value,
  onChange,
  min,
  max,
  step,
}: {
  label: string
  value: number
  onChange: (v: number) => void
  min: number
  max: number
  step?: number | 'any'
}) {
  return (
    <Field>
      <Label>{label}</Label>
      <Input
        type="number"
        min={min}
        max={max}
        step={step}
        value={Number.isNaN(value) ? '' : value}
        onChange={(e) => onChange(e.target.valueAsNumber)}
        required
      />
    </Field>
  )
}

type AugmentationKey = Exclude<keyof Augmentation, '$typeName' | '$unknown'>

const augmentationFields: {
  key: AugmentationKey
  label: string
  min: number
  max: number
  step?: number | 'any'
}[] = [
  { key: 'degrees', label: 'Rotation (°)', min: 0, max: 180 },
  { key: 'translate', label: 'Shift', min: 0, max: 0.9, step: 'any' },
  { key: 'scale', label: 'Zoom', min: 0, max: 0.9, step: 'any' },
  { key: 'aspect', label: 'Stretch', min: 1, max: 2, step: 'any' },
  { key: 'shear', label: 'Shear (°)', min: 0, max: 45 },
  { key: 'perspective', label: 'Perspective', min: 0, max: 0.001, step: 'any' },
  { key: 'hsvH', label: 'Hue', min: 0, max: 1, step: 'any' },
  { key: 'hsvS', label: 'Saturation', min: 0, max: 1, step: 'any' },
  { key: 'hsvV', label: 'Brightness', min: 0, max: 1, step: 'any' },
  { key: 'mosaic', label: 'Mosaic', min: 0, max: 1, step: 'any' },
  { key: 'fliplr', label: 'Mirror', min: 0, max: 1, step: 'any' },
  { key: 'flipud', label: 'Upside down', min: 0, max: 1, step: 'any' },
  { key: 'underwater', label: 'Underwater', min: 0, max: 1, step: 'any' },
  { key: 'shadows', label: 'Light and shadow', min: 0, max: 1, step: 'any' },
  { key: 'noise', label: 'Noise', min: 0, max: 1, step: 'any' },
]

function AugmentationFields({
  value,
  onChange,
}: {
  value: Record<AugmentationKey, number>
  onChange: (v: Record<AugmentationKey, number>) => void
}) {
  const [open, setOpen] = useState(false)
  return (
    <Fieldset>
      <Legend>
        <button
          type="button"
          aria-expanded={open}
          onClick={() => setOpen((o) => !o)}
          className="flex items-center gap-2 rounded-sm focus:outline-none focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-500"
        >
          <ChevronRightIcon
            className={clsx(
              'size-4 fill-zinc-400 transition-transform duration-150 dark:fill-zinc-500',
              open && 'rotate-90',
            )}
          />
          Augmentation
        </button>
      </Legend>
      {open && (
        <div className="mt-6 grid gap-6 sm:grid-cols-3">
          {augmentationFields.map((f) => (
            <NumberField
              key={f.key}
              label={f.label}
              value={value[f.key]}
              onChange={(v) => onChange({ ...value, [f.key]: v })}
              min={f.min}
              max={f.max}
              step={f.step}
            />
          ))}
        </div>
      )}
    </Fieldset>
  )
}

function augmentationValues(a: Augmentation | undefined) {
  const out = {} as Record<AugmentationKey, number>
  for (const f of augmentationFields) out[f.key] = float32(a?.[f.key] ?? 0)
  return out
}

function TrainForm({
  defaults,
  runs,
  onClose,
}: {
  defaults: TrainConfig
  runs: TrainingRun[]
  onClose: () => void
}) {
  const navigate = useNavigate()
  const dataset = defaults.dataset
  const [size, setSize] = useState(defaults.size)
  const [baseRunId, setBaseRunId] = useState(0n)
  const [epochs, setEpochs] = useState(defaults.epochs)
  const [imageSize, setImageSize] = useState(defaults.imageSize)
  const [batch, setBatch] = useState(defaults.batch)
  const [valFraction, setValFraction] = useState(dataset?.valFraction ?? 0.2)
  const [stride, setStride] = useState(dataset?.stride ?? 1)
  const [dedup, setDedup] = useState(dataset?.dedup ?? true)
  const [dedupDistance, setDedupDistance] = useState(dataset?.dedupDistance || 4)
  const [augmentation, setAugmentation] = useState(() => augmentationValues(defaults.augmentation))
  const finished = runs.filter((r) => r.status === TrainingStatus.SUCCEEDED)

  const start = useMutation(PipelineService.method.startTraining, {
    onSuccess: async (res) => {
      await invalidateService(PipelineService)
      onClose()
      if (res.run) navigate(`/admin/models/${res.run.id}`)
    },
  })

  return (
    <form
      onSubmit={(e) => {
        e.preventDefault()
        start.mutate({
          config: create(TrainConfigSchema, {
            size,
            baseRunId,
            epochs,
            imageSize,
            batch,
            dataset: create(ExportOptionsSchema, {
              valFraction: Math.round(valFraction * 100) / 100,
              stride,
              dedup,
              dedupDistance: dedup ? dedupDistance : 0,
            }),
            augmentation: create(AugmentationSchema, augmentation),
          }),
        })
      }}
    >
      <DialogTitle>Train a model</DialogTitle>
      <DialogBody>
        <FieldGroup>
          <div className="grid gap-6 sm:grid-cols-2">
            <Field>
              <Label>Start from</Label>
              <Select
                value={String(baseRunId)}
                onChange={(e) => setBaseRunId(BigInt(e.target.value))}
              >
                <option value="0">COCO weights</option>
                {finished.map((r) => (
                  <option key={String(r.id)} value={String(r.id)}>
                    {runName(r.id)}
                    {r.filename ? ` · ${r.filename}` : ''}
                    {r.pinned ? ' (pinned)' : ''}
                  </option>
                ))}
              </Select>
            </Field>
            <Field disabled={baseRunId !== 0n}>
              <Label>Model</Label>
              <Select value={size} onChange={(e) => setSize(Number(e.target.value) as ModelSize)}>
                {[ModelSize.NANO, ModelSize.SMALL].map((s) => (
                  <option key={s} value={s}>
                    {sizeNames[s]}
                  </option>
                ))}
              </Select>
            </Field>
          </div>
          <div className="grid gap-6 sm:grid-cols-3">
            <NumberField label="Epochs" value={epochs} onChange={setEpochs} min={1} max={1000} />
            <NumberField
              label="Image size"
              value={imageSize}
              onChange={setImageSize}
              min={320}
              max={1920}
              step={32}
            />
            <NumberField label="Batch size" value={batch} onChange={setBatch} min={1} max={256} />
          </div>
          <Fieldset>
            <Legend>Dataset</Legend>
            <div className="mt-6 grid gap-6 sm:grid-cols-2">
              <NumberField
                label="Validation share"
                value={valFraction}
                onChange={setValFraction}
                min={0.05}
                max={0.5}
                step={0.05}
              />
              <NumberField
                label="Frame stride"
                value={stride}
                onChange={setStride}
                min={1}
                max={100}
              />
            </div>
            <div className="mt-6 grid items-end gap-6 sm:grid-cols-2">
              <CheckboxField>
                <Checkbox checked={dedup} onChange={setDedup} color="sky" />
                <Label>Drop near-duplicate frames</Label>
              </CheckboxField>
              {dedup && (
                <NumberField
                  label="Duplicate threshold"
                  value={dedupDistance}
                  onChange={setDedupDistance}
                  min={1}
                  max={16}
                />
              )}
            </div>
          </Fieldset>
          <AugmentationFields value={augmentation} onChange={setAugmentation} />
          <FormError error={start.error} />
        </FieldGroup>
      </DialogBody>
      <DialogActions>
        <Button plain onClick={onClose}>
          Cancel
        </Button>
        <Button type="submit" color="sky" disabled={start.isPending}>
          Train
        </Button>
      </DialogActions>
    </form>
  )
}

export function TrainDialog({
  open,
  onClose,
  defaults,
  runs,
}: {
  open: boolean
  onClose: () => void
  defaults: TrainConfig | undefined
  runs: TrainingRun[]
}) {
  return (
    <Dialog open={open} onClose={onClose} size="2xl">
      {open && defaults && <TrainForm defaults={defaults} runs={runs} onClose={onClose} />}
    </Dialog>
  )
}
