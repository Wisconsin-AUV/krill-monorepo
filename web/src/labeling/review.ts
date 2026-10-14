import { AnnotationStatus, type Annotation } from '@/gen/krill/v1/annotation_pb'
import type { Frame } from '@/gen/krill/v1/clip_pb'
import { idKey } from './useLabelStore'

export interface TrackReview {
  // frame id -> ids of tracks whose proposal on that frame the server flagged as drifted
  drift: Map<string, Set<string>>
  // Tracks whose last box is still a proposal. Exports skip them.
  unconfirmed: Set<string>
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
      // Accepting or editing a box verifies it locally without clearing the server's flag.
      if (a.drift && a.status === AnnotationStatus.PROPOSED) {
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
