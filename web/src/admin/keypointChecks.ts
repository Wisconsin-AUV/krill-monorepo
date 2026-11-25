import type { KeypointCheck, KeypointCheckType } from '@/gen/krill/v1/label_pb'

export function anyChecking(types: KeypointCheckType[] | undefined) {
  return types?.some((t) => t.running || t.runningTracking) ?? false
}

// Errors under 10 px keep a decimal, since a pixel or two is the difference
// between good fits.
export function formatPixels(px: number) {
  return `${px < 10 ? px.toFixed(1) : Math.round(px)} px`
}

export function foundShare(c: KeypointCheck) {
  return c.boxCount > 0 ? c.foundCount / c.boxCount : 0
}
