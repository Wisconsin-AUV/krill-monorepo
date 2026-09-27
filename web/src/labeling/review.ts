import { AnnotationStatus, type Annotation, type Box } from '@/gen/krill/v1/annotation_pb'
import type { Frame } from '@/gen/krill/v1/clip_pb'
import { idKey } from './useLabelStore'

// A proposal that overlaps the track's previous box less than this, or grows
// or shrinks by more than this factor, has likely slid off its object.
const MIN_IOU = 0.5
const MAX_AREA_RATIO = 1.5

export interface TrackReview {
  // frame id -> ids of tracks whose proposal on that frame looks drifted
  drift: Map<string, Set<string>>
  // Tracks whose last box is still a proposal. Exports skip them.
  unconfirmed: Set<string>
}

function area(b: Box): number {
  return b.width * b.height
}

function iou(a: Box, b: Box): number {
  const w = Math.min(a.x + a.width, b.x + b.width) - Math.max(a.x, b.x)
  const h = Math.min(a.y + a.height, b.y + b.height) - Math.max(a.y, b.y)
  if (w <= 0 || h <= 0) return 0
  const overlap = w * h
  return overlap / (area(a) + area(b) - overlap)
}

function drifted(prev: Box, next: Box): boolean {
  const ratio = area(next) / area(prev)
  return iou(prev, next) < MIN_IOU || ratio > MAX_AREA_RATIO || ratio < 1 / MAX_AREA_RATIO
}

export function reviewTracks(
  frames: Frame[],
  boxes: Record<string, Record<string, Annotation>>,
): TrackReview {
  const drift = new Map<string, Set<string>>()
  const last = new Map<string, Annotation>()
  for (const f of frames) {
    const frameKey = idKey(f.id)
    for (const [trackKey, a] of Object.entries(boxes[frameKey] ?? {})) {
      const prev = last.get(trackKey)?.box
      if (prev && a.box && a.status === AnnotationStatus.PROPOSED && drifted(prev, a.box)) {
        const set = drift.get(frameKey) ?? new Set()
        drift.set(frameKey, set.add(trackKey))
      }
      last.set(trackKey, a)
    }
  }
  const unconfirmed = new Set<string>()
  for (const [trackKey, a] of last) {
    if (a.status === AnnotationStatus.PROPOSED) unconfirmed.add(trackKey)
  }
  return { drift, unconfirmed }
}
