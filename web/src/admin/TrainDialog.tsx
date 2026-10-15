import { create } from '@bufbuild/protobuf'
import { useMutation } from '@connectrpc/connect-query'
import { useState } from 'react'
import { useNavigate } from 'react-router'
import { Button } from '@/components/ui/Button'
import { FormError } from '@/components/ui/Callout'
import { Checkbox, CheckboxField } from '@/components/ui/Checkbox'
import { Dialog, DialogActions, DialogBody, DialogTitle } from '@/components/ui/Dialog'
import { Field, FieldGroup, Fieldset, Label, Legend } from '@/components/ui/Fieldset'
import { Input } from '@/components/ui/Input'
import { Select } from '@/components/ui/Select'
import { ExportOptionsSchema } from '@/gen/krill/v1/export_pb'
import {
  ModelSize,
  PipelineService,
  TrainConfigSchema,
  TrainingStatus,
  type TrainConfig,
  type TrainingRun,
} from '@/gen/krill/v1/pipeline_pb'
import { invalidateService } from '@/lib/queryClient'
import { runName, sizeNames } from './pipeline'

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
