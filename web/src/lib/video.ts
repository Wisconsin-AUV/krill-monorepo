import type { BadgeColor } from '@/components/ui/Badge'
import { SplitAssignment, VideoStatus, type Video } from '@/gen/krill/v1/video_pb'

export const statusMeta: Record<VideoStatus, { label: string; color: BadgeColor }> = {
  [VideoStatus.UNSPECIFIED]: { label: 'Unknown', color: 'zinc' },
  [VideoStatus.UPLOADING]: { label: 'Uploading', color: 'zinc' },
  [VideoStatus.QUEUED]: { label: 'Queued', color: 'sky' },
  [VideoStatus.PROCESSING]: { label: 'Processing', color: 'sky' },
  [VideoStatus.READY]: { label: 'Ready', color: 'green' },
  [VideoStatus.FAILED]: { label: 'Failed', color: 'red' },
}

export const splitOptions = [
  {
    value: SplitAssignment.AUTO,
    label: 'Auto',
    description: 'Balanced between train and val on export',
  },
  { value: SplitAssignment.TRAIN, label: 'Train', description: 'Always in the training set' },
  { value: SplitAssignment.VAL, label: 'Validation', description: 'Always in the validation set' },
]

export function splitLabel(split: SplitAssignment): string {
  return splitOptions.find((o) => o.value === split)?.label ?? 'Auto'
}

export const extractFpsOptions = [
  { value: 0, label: 'Native frame rate' },
  { value: 30, label: '30 fps' },
  { value: 15, label: '15 fps' },
  { value: 10, label: '10 fps' },
  { value: 5, label: '5 fps' },
  { value: 2, label: '2 fps' },
]

export function isIngesting(v: Video): boolean {
  return v.status === VideoStatus.QUEUED || v.status === VideoStatus.PROCESSING
}
