import { KeypointVisibility } from '@/gen/krill/v1/annotation_pb'
import type { LabelType } from '@/gen/krill/v1/label_pb'
import {
  derivedIndexes,
  hasKeypoints,
  isLabeled,
  nearestUnplaced,
  nextUnlabeled,
  padKeypoints,
  setKeypoint,
  stepIndex,
  type KeypointValue,
} from '@/lib/keypoints'
import type { HotkeyMap } from '@/workspace/useHotkeys'
import { idKey, useLabelStore, type KeypointFocus } from './useLabelStore'

const labels = useLabelStore.getState

const onBox = (f: KeypointFocus | null, trackId: bigint, frameId: bigint): f is KeypointFocus =>
  !!f && f.trackId === trackId && f.frameId === frameId

export function boxKeypoints(type: LabelType, trackId: bigint, frameId: bigint) {
  const a = labels().boxes[idKey(frameId)]?.[idKey(trackId)]
  return a ? padKeypoints(a.box?.keypoints ?? [], type.keypoints.length) : undefined
}

export function activeKeypoint(
  focus: KeypointFocus | null,
  kps: readonly KeypointValue[],
  type: LabelType,
  trackId: bigint,
  frameId: bigint,
): number | null {
  return onBox(focus, trackId, frameId) ? focus.index : nextUnlabeled(kps, type, -1)
}

// The keypoint a click at a normalized point places: the one the labeler
// picked, else the unplaced corner nearest it once a quad's placed corners
// say where the rest are, else the active one. aspect is the frame's width
// over its height.
export function clickedKeypoint(
  focus: KeypointFocus | null,
  kps: readonly KeypointValue[],
  type: LabelType,
  trackId: bigint,
  frameId: bigint,
  point: { x: number; y: number } | null,
  aspect: number,
): number | null {
  const active = activeKeypoint(focus, kps, type, trackId, frameId)
  if (!point || (onBox(focus, trackId, frameId) && !focus.advanced)) return active
  return nearestUnplaced(kps, type, point, aspect) ?? active
}

const outside = (p: { x: number; y: number }) => p.x < 0 || p.x > 1 || p.y < 0 || p.y > 1

// Places the keypoint clickedKeypoint names at a normalized point and moves
// on to the next unlabeled one. False when there is none to place.
export function placeKeypoint(
  type: LabelType,
  trackId: bigint,
  frameId: bigint,
  point: { x: number; y: number },
  hidden: boolean,
  aspect: number,
): boolean {
  const kps = boxKeypoints(type, trackId, frameId)
  if (!kps) return false
  const focus = labels().keypointFocus
  const index = clickedKeypoint(focus, kps, type, trackId, frameId, point, aspect)
  if (index === null) return false
  const visibility =
    hidden || outside(point) ? KeypointVisibility.HIDDEN : KeypointVisibility.VISIBLE
  const next = setKeypoint(kps, type, index, { ...point, visibility })
  labels().focusKeypoint(
    { trackId, frameId, index: nextUnlabeled(next, type, index), advanced: true },
    { trackId, frameId, index },
  )
  void labels().setKeypoints(trackId, frameId, next)
  return true
}

export function moveKeypoint(
  type: LabelType,
  trackId: bigint,
  frameId: bigint,
  index: number,
  point: { x: number; y: number },
) {
  const kps = boxKeypoints(type, trackId, frameId)
  if (!kps || !isLabeled(kps[index])) return
  const visibility = outside(point) ? KeypointVisibility.HIDDEN : kps[index].visibility
  labels().focusKeypoint(null, { trackId, frameId, index })
  void labels().setKeypoints(
    trackId,
    frameId,
    setKeypoint(kps, type, index, { ...point, visibility }),
  )
}

// The keypoint o and x act on: the active one once placed, else the one
// placed last.
function target(type: LabelType, trackId: bigint, frameId: bigint) {
  const kps = boxKeypoints(type, trackId, frameId)
  if (!kps) return undefined
  const { keypointFocus, lastKeypoint } = labels()
  const derived = derivedIndexes(type.quads)
  const candidates = [
    activeKeypoint(keypointFocus, kps, type, trackId, frameId),
    onBox(lastKeypoint, trackId, frameId) ? lastKeypoint.index : null,
  ]
  const index = candidates.find((i) => i !== null && !derived.has(i) && isLabeled(kps[i]))
  return index === undefined || index === null ? undefined : { kps, index }
}

export function keypointKeys(
  type: LabelType | undefined,
  trackId: bigint | null,
  frameId: bigint | undefined,
): HotkeyMap {
  if (!hasKeypoints(type) || trackId === null || frameId === undefined) return {}
  if (!labels().boxes[idKey(frameId)]?.[idKey(trackId)]) return {}
  const step = (delta: number) => {
    const kps = boxKeypoints(type, trackId, frameId) ?? []
    const from = activeKeypoint(labels().keypointFocus, kps, type, trackId, frameId)
    labels().focusKeypoint({ trackId, frameId, index: stepIndex(type, from, delta) })
  }
  // Every digit is taken while a box with keypoints is selected, so one past
  // the type's keypoints or on a derived center does nothing rather than
  // falling through to picking a label type, which would drop the selection.
  const derived = derivedIndexes(type.quads)
  const pick = Object.fromEntries(
    Array.from({ length: 9 }, (_, index) => [
      String(index + 1),
      () => {
        if (index < type.keypoints.length && !derived.has(index))
          labels().focusKeypoint({ trackId, frameId, index })
      },
    ]),
  )
  return {
    ...pick,
    o: () => {
      const t = target(type, trackId, frameId)
      if (!t) return
      const k = t.kps[t.index]
      const visibility =
        k.visibility === KeypointVisibility.HIDDEN
          ? KeypointVisibility.VISIBLE
          : KeypointVisibility.HIDDEN
      void labels().setKeypoints(
        trackId,
        frameId,
        setKeypoint(t.kps, type, t.index, { ...k, visibility }),
      )
    },
    x: () => {
      const t = target(type, trackId, frameId)
      if (!t) return
      labels().focusKeypoint({ trackId, frameId, index: t.index })
      void labels().setKeypoints(
        trackId,
        frameId,
        setKeypoint(t.kps, type, t.index, {
          x: 0,
          y: 0,
          visibility: KeypointVisibility.UNSPECIFIED,
        }),
      )
    },
    w: () => step(1),
    q: () => step(-1),
  }
}
