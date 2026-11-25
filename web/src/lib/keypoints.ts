import { KeypointVisibility } from '@/gen/krill/v1/annotation_pb'
import type { KeypointQuad, LabelType } from '@/gen/krill/v1/label_pb'
import type { Point } from './obb'

// Normalized to the frame, like Box.keypoints.
export interface KeypointValue {
  x: number
  y: number
  visibility: KeypointVisibility
}

const MIN_COORD = -1
const MAX_COORD = 2

export const unlabeled = (): KeypointValue => ({
  x: 0,
  y: 0,
  visibility: KeypointVisibility.UNSPECIFIED,
})

export const isLabeled = (k: KeypointValue | undefined): k is KeypointValue =>
  !!k && k.visibility !== KeypointVisibility.UNSPECIFIED

export const hasKeypoints = (type: LabelType | undefined): type is LabelType =>
  (type?.keypoints.length ?? 0) > 0

const clamp = (v: number) => Math.min(Math.max(v, MIN_COORD), MAX_COORD)

// Boxes saved before keypoints were appended to their type have fewer.
export function padKeypoints(kps: readonly KeypointValue[], count: number): KeypointValue[] {
  const out = kps.slice(0, count).map((k) => ({ x: k.x, y: k.y, visibility: k.visibility }))
  while (out.length < count) out.push(unlabeled())
  return out
}

// Where the diagonals corner0-corner2 and corner1-corner3 cross, which is the
// quad's center in perspective.
export function quadCenter(c: readonly Point[]): Point {
  const r = { x: c[2].x - c[0].x, y: c[2].y - c[0].y }
  const q = { x: c[3].x - c[1].x, y: c[3].y - c[1].y }
  const denom = r.x * q.y - r.y * q.x
  if (Math.abs(denom) < 1e-12) {
    return {
      x: (c[0].x + c[1].x + c[2].x + c[3].x) / 4,
      y: (c[0].y + c[1].y + c[2].y + c[3].y) / 4,
    }
  }
  const t = ((c[1].x - c[0].x) * q.y - (c[1].y - c[0].y) * q.x) / denom
  return { x: c[0].x + t * r.x, y: c[0].y + t * r.y }
}

export function derivedIndexes(quads: readonly KeypointQuad[]): Set<number> {
  return new Set(quads.flatMap((q) => (q.center === undefined ? [] : [q.center])))
}

export function withCenters(
  kps: readonly KeypointValue[],
  quads: readonly KeypointQuad[],
): KeypointValue[] {
  const out = [...kps]
  for (const q of quads) {
    if (q.center === undefined || q.center >= out.length || q.corners.length !== 4) continue
    const corners = q.corners.map((i) => out[i])
    if (!corners.every(isLabeled)) {
      out[q.center] = unlabeled()
      continue
    }
    const hidden = corners.every((k) => k.visibility === KeypointVisibility.HIDDEN)
    out[q.center] = {
      ...quadCenter(corners),
      visibility: hidden ? KeypointVisibility.HIDDEN : KeypointVisibility.VISIBLE,
    }
  }
  return out
}

// Sets one keypoint of a box and recomputes the derived centers. Empty when
// nothing is left labeled, as boxes with no keypoints are stored.
export function setKeypoint(
  kps: readonly KeypointValue[],
  type: LabelType,
  index: number,
  value: KeypointValue,
): KeypointValue[] {
  const out = padKeypoints(kps, type.keypoints.length)
  out[index] = isLabeled(value)
    ? { x: clamp(value.x), y: clamp(value.y), visibility: value.visibility }
    : unlabeled()
  const centered = withCenters(out, type.quads)
  return centered.some(isLabeled) ? centered : []
}

// The next keypoint after from that a labeler places, wrapping around, or
// null when every one is labeled.
export function nextUnlabeled(
  kps: readonly KeypointValue[],
  type: LabelType,
  from: number,
): number | null {
  const n = type.keypoints.length
  const derived = derivedIndexes(type.quads)
  for (let step = 1; step <= n; step++) {
    const i = (((from + step) % n) + n) % n
    if (!derived.has(i) && !isLabeled(kps[i])) return i
  }
  return null
}

export function stepIndex(type: LabelType, from: number | null, delta: number): number | null {
  const n = type.keypoints.length
  const derived = derivedIndexes(type.quads)
  const start = from ?? (delta > 0 ? -1 : 0)
  for (let step = 1; step <= n; step++) {
    const i = (((start + delta * step) % n) + n) % n
    if (!derived.has(i)) return i
  }
  return null
}

export function insidePolygon(p: Point, poly: readonly Point[]): boolean {
  let inside = false
  for (let i = 0, j = poly.length - 1; i < poly.length; j = i++) {
    const a = poly[i]
    const b = poly[j]
    if (a.y > p.y !== b.y > p.y && p.x < ((b.x - a.x) * (p.y - a.y)) / (b.y - a.y) + a.x)
      inside = !inside
  }
  return inside
}

// The quad SAM fills from a Shift+click: the smallest labeled one clicked
// inside, so a wrong one can be filled again, then the printed one while it
// is incomplete, then the one being placed, then the first missing a corner.
export function promptQuad(
  type: LabelType,
  kps: readonly KeypointValue[],
  active: number | null,
  point: Point,
): number {
  const quads = type.quads
  const labeled = (i: number) => quads[i].corners.every((c) => isLabeled(kps[c]))
  const corners = (i: number) => quads[i].corners.map((c) => kps[c])
  const under = quads
    .map((_, i) => i)
    .filter((i) => labeled(i) && insidePolygon(point, corners(i)))
    .sort((a, b) => polygonArea(corners(a)) - polygonArea(corners(b)))
  if (under.length > 0) return under[0]
  const printed = quads.findIndex((q, i) => q.reference && !labeled(i))
  if (printed >= 0) return printed
  const holding = quads.findIndex((q) => active !== null && q.corners.includes(active))
  if (holding >= 0) return holding
  return Math.max(
    quads.findIndex((_, i) => !labeled(i)),
    0,
  )
}

function polygonArea(poly: readonly Point[]): number {
  let sum = 0
  for (let i = 0, j = poly.length - 1; i < poly.length; j = i++)
    sum += poly[j].x * poly[i].y - poly[i].x * poly[j].y
  return Math.abs(sum) / 2
}

const UNIT_SQUARE: readonly Point[] = [
  { x: 0, y: 0 },
  { x: 1, y: 0 },
  { x: 1, y: 1 },
  { x: 0, y: 1 },
]

// Where a quad's missing corners would be from its placed ones: the fourth
// of a parallelogram through three, or a square on two. Corners go clockwise
// on screen, which is what tells the side of two placed corners the others
// are on. Empty when fewer than two are placed.
function guessCorners(corners: readonly (Point | null)[]): Map<number, Point> {
  const placed = [0, 1, 2, 3].filter((i) => corners[i])
  const missing = [0, 1, 2, 3].filter((i) => !corners[i])
  const at = (i: number) => corners[i % 4] as Point
  const out = new Map<number, Point>()
  if (placed.length === 3) {
    const k = missing[0]
    out.set(k, {
      x: at(k + 1).x + at(k + 3).x - at(k + 2).x,
      y: at(k + 1).y + at(k + 3).y - at(k + 2).y,
    })
  } else if (placed.length === 2) {
    // The similarity taking the unit square's corners to the placed two,
    // as complex numbers: z = z0 + s * (u - u0).
    const [i, j] = placed
    const du = { x: UNIT_SQUARE[j].x - UNIT_SQUARE[i].x, y: UNIT_SQUARE[j].y - UNIT_SQUARE[i].y }
    const dz = { x: at(j).x - at(i).x, y: at(j).y - at(i).y }
    const n = du.x * du.x + du.y * du.y
    const s = { x: (dz.x * du.x + dz.y * du.y) / n, y: (dz.y * du.x - dz.x * du.y) / n }
    for (const k of missing) {
      const u = { x: UNIT_SQUARE[k].x - UNIT_SQUARE[i].x, y: UNIT_SQUARE[k].y - UNIT_SQUARE[i].y }
      out.set(k, { x: at(i).x + s.x * u.x - s.y * u.y, y: at(i).y + s.x * u.y + s.y * u.x })
    }
  }
  return out
}

// The unlabeled quad corner whose guessed spot is nearest a point, or null
// when no quad has enough corners placed to guess from. aspect is the
// frame's width over its height, since the square a guess assumes is one in
// pixels.
export function nearestUnplaced(
  kps: readonly KeypointValue[],
  type: LabelType,
  point: Point,
  aspect: number,
): number | null {
  const px = (p: Point) => ({ x: p.x * aspect, y: p.y })
  const target = px(point)
  let best: number | null = null
  let bestDist = Infinity
  for (const q of type.quads) {
    if (q.corners.length !== 4) continue
    const corners = q.corners.map((c) => (isLabeled(kps[c]) ? px(kps[c]) : null))
    for (const [i, p] of guessCorners(corners)) {
      const d = (p.x - target.x) ** 2 + (p.y - target.y) ** 2
      if (d < bestDist) {
        best = q.corners[i]
        bestDist = d
      }
    }
  }
  return best
}

// Maps the unit square onto a quad, (0,0), (1,0), (1,1), (0,1) to its corners
// in order (Heckbert's square-to-quad projection).
export function squareToQuad(c: readonly Point[]): (u: number, v: number) => Point {
  const sx = c[0].x - c[1].x + c[2].x - c[3].x
  const sy = c[0].y - c[1].y + c[2].y - c[3].y
  let g = 0
  let h = 0
  if (Math.abs(sx) > 1e-12 || Math.abs(sy) > 1e-12) {
    const dx1 = c[1].x - c[2].x
    const dx2 = c[3].x - c[2].x
    const dy1 = c[1].y - c[2].y
    const dy2 = c[3].y - c[2].y
    const det = dx1 * dy2 - dx2 * dy1
    g = (sx * dy2 - dx2 * sy) / det
    h = (dx1 * sy - sx * dy1) / det
  }
  const a = c[1].x - c[0].x + g * c[1].x
  const b = c[3].x - c[0].x + h * c[3].x
  const d = c[1].y - c[0].y + g * c[1].y
  const e = c[3].y - c[0].y + h * c[3].y
  return (u, v) => {
    const w = g * u + h * v + 1
    return { x: (a * u + b * v + c[0].x) / w, y: (d * u + e * v + c[0].y) / w }
  }
}
