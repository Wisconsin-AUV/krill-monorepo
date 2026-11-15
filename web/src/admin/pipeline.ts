import { timestampDate, type Timestamp } from '@bufbuild/protobuf/wkt'
import type { BadgeColor } from '@/components/ui/Badge'
import {
  ModelSize,
  ModelTrialStatus,
  PrelabelStatus,
  TrainingStatus,
  TrainingTrigger,
  type ModelTrial,
  type TrainingRun,
} from '@/gen/krill/v1/pipeline_pb'
import type { Augmentation } from '@/gen/krill/v1/export_pb'
import { formatDuration } from '@/lib/format'

export const trainingStatus: Record<TrainingStatus, { label: string; color: BadgeColor }> = {
  [TrainingStatus.UNSPECIFIED]: { label: 'Unknown', color: 'zinc' },
  [TrainingStatus.QUEUED]: { label: 'Queued', color: 'zinc' },
  [TrainingStatus.PREPARING]: { label: 'Preparing', color: 'sky' },
  [TrainingStatus.TRAINING]: { label: 'Training', color: 'sky' },
  [TrainingStatus.EVALUATING]: { label: 'Evaluating', color: 'sky' },
  [TrainingStatus.SUCCEEDED]: { label: 'Done', color: 'green' },
  [TrainingStatus.FAILED]: { label: 'Failed', color: 'red' },
  [TrainingStatus.CANCELLED]: { label: 'Cancelled', color: 'zinc' },
}

export const prelabelStatus: Record<PrelabelStatus, { label: string; color: BadgeColor }> = {
  [PrelabelStatus.UNSPECIFIED]: { label: 'Unknown', color: 'zinc' },
  [PrelabelStatus.QUEUED]: { label: 'Queued', color: 'zinc' },
  [PrelabelStatus.RUNNING]: { label: 'Running', color: 'sky' },
  [PrelabelStatus.SUCCEEDED]: { label: 'Done', color: 'green' },
  [PrelabelStatus.FAILED]: { label: 'Failed', color: 'red' },
  [PrelabelStatus.CANCELLED]: { label: 'Cancelled', color: 'zinc' },
}

export const sizeNames: Record<ModelSize, string> = {
  [ModelSize.UNSPECIFIED]: '',
  [ModelSize.NANO]: 'YOLO nano',
  [ModelSize.SMALL]: 'YOLO small',
}

export function uploaded(r: TrainingRun) {
  return r.trigger === TrainingTrigger.UPLOAD
}

// Runs fine-tuned from uploaded weights have no size.
export function modelName(r: TrainingRun) {
  return uploaded(r) ? 'Uploaded' : sizeNames[r.config?.size ?? 0] || 'YOLO'
}

export function trainingActive(r: TrainingRun) {
  return (
    r.status === TrainingStatus.QUEUED ||
    r.status === TrainingStatus.PREPARING ||
    r.status === TrainingStatus.TRAINING ||
    r.status === TrainingStatus.EVALUATING
  )
}

export function prelabelActive(s: PrelabelStatus) {
  return s === PrelabelStatus.QUEUED || s === PrelabelStatus.RUNNING
}

export function percent(v: number | undefined) {
  return v === undefined ? '—' : `${(v * 100).toFixed(1)}`
}

export function runName(id: bigint) {
  return `Run ${id}`
}

// How long a run took, or has taken so far.
export function elapsed(
  start: Timestamp | undefined,
  end: Timestamp | undefined,
  now: number,
): string | null {
  if (!start) return null
  const to = end ? timestampDate(end).getTime() : now
  return formatDuration(to - timestampDate(start).getTime())
}

export const trialStatus: Record<ModelTrialStatus, { label: string; color: BadgeColor }> = {
  [ModelTrialStatus.UNSPECIFIED]: { label: 'Unknown', color: 'zinc' },
  [ModelTrialStatus.QUEUED]: { label: 'Queued', color: 'zinc' },
  [ModelTrialStatus.RUNNING]: { label: 'Running', color: 'sky' },
  [ModelTrialStatus.SUCCEEDED]: { label: 'Done', color: 'green' },
  [ModelTrialStatus.FAILED]: { label: 'Failed', color: 'red' },
  [ModelTrialStatus.CANCELLED]: { label: 'Cancelled', color: 'zinc' },
}

export function trialActive(t: ModelTrial) {
  return t.status === ModelTrialStatus.QUEUED || t.status === ModelTrialStatus.RUNNING
}

// Floats come back from protobuf as float32, so 0.015 would show as
// 0.014999999664723873.
export function float32(v: number) {
  return Number(v.toPrecision(3))
}

export type AugmentationKey = Exclude<keyof Augmentation, '$typeName' | '$unknown'>
export type AugmentationValues = Record<AugmentationKey, number>

export const augmentationFields: {
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
  { key: 'distractors', label: 'Distractors', min: 0, max: 1, step: 'any' },
]

export function augmentationValues(a: Augmentation | undefined) {
  const out = {} as AugmentationValues
  for (const f of augmentationFields) out[f.key] = float32(a?.[f.key] ?? 0)
  return out
}

export function augmentationSummary(a: Augmentation) {
  const pct = (p: number) => `${Math.round(p * 100)}%`
  const parts = [
    a.degrees > 0 && `±${float32(a.degrees)}°`,
    a.translate > 0 && `shift ${float32(a.translate)}`,
    a.scale > 0 && `zoom ${float32(a.scale)}`,
    a.aspect > 1 && `stretch ${float32(a.aspect)}`,
    a.shear > 0 && `shear ${float32(a.shear)}°`,
    a.perspective > 0 && `perspective ${float32(a.perspective)}`,
    (a.hsvH > 0 || a.hsvS > 0 || a.hsvV > 0) &&
      `HSV ${float32(a.hsvH)}/${float32(a.hsvS)}/${float32(a.hsvV)}`,
    a.mosaic > 0 && `mosaic ${pct(a.mosaic)}`,
    a.fliplr > 0 && `mirror ${pct(a.fliplr)}`,
    a.flipud > 0 && `upside down ${pct(a.flipud)}`,
    a.underwater > 0 && `underwater ${pct(a.underwater)}`,
    a.shadows > 0 && `light and shadow ${pct(a.shadows)}`,
    a.noise > 0 && `noise ${pct(a.noise)}`,
    a.distractors > 0 && `distractors ${pct(a.distractors)}`,
  ]
  return parts.filter(Boolean).join(' · ') || 'None'
}
