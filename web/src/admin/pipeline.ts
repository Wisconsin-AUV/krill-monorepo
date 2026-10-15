import { timestampDate, type Timestamp } from '@bufbuild/protobuf/wkt'
import type { BadgeColor } from '@/components/ui/Badge'
import {
  ModelSize,
  PrelabelStatus,
  TrainingStatus,
  type TrainingRun,
} from '@/gen/krill/v1/pipeline_pb'
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
