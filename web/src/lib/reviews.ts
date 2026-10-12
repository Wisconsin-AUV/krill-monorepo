import type { BadgeColor } from '@/components/ui/Badge'
import { ReviewReason, ReviewResolution } from '@/gen/krill/v1/review_pb'

export const reasons: Record<ReviewReason, { key: string; label: string; color: BadgeColor }> = {
  [ReviewReason.UNSPECIFIED]: { key: 'all', label: 'All', color: 'zinc' },
  [ReviewReason.FLAGGED]: { key: 'flagged', label: 'Flagged', color: 'amber' },
  [ReviewReason.LOW_ACCURACY]: { key: 'accuracy', label: 'Low accuracy', color: 'orange' },
  [ReviewReason.DRIFT]: { key: 'drift', label: 'Drift', color: 'rose' },
}

export const reasonTabs = Object.entries(reasons).map(([reason, r]) => ({
  reason: Number(reason) as ReviewReason,
  key: r.key,
  label: r.label,
}))

export function reasonFromKey(key: string | null): ReviewReason {
  return reasonTabs.find((t) => t.key === key)?.reason ?? ReviewReason.UNSPECIFIED
}

export const resolutions = [
  { resolution: ReviewResolution.ACCEPTED, label: 'Accepted' },
  { resolution: ReviewResolution.FIXED, label: 'Fixed' },
  { resolution: ReviewResolution.DISMISSED, label: 'Dismissed' },
] as const
