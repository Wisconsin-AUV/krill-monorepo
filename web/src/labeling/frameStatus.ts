import type { BadgeColor } from '@/components/ui/Badge'
import { FrameStatus } from '@/gen/krill/v1/annotation_pb'

export function frameState(status: FrameStatus | undefined, boxCount: number) {
  if (status === FrameStatus.LABELED)
    return { label: 'Labeled', color: 'green' as BadgeColor, bar: 'bg-emerald-500/80' }
  if (status === FrameStatus.EMPTY)
    return { label: 'Empty', color: 'zinc' as BadgeColor, bar: 'bg-zinc-500/80' }
  if (boxCount > 0)
    return { label: 'In progress', color: 'amber' as BadgeColor, bar: 'bg-amber-400/60' }
  return { label: 'Unlabeled', color: 'zinc' as BadgeColor, bar: '' }
}
