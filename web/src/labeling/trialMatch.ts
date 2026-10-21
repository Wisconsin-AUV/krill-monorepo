import {
  AnnotationStatus,
  FrameStatus,
  type Annotation,
  type Track,
} from '@/gen/krill/v1/annotation_pb'
import type { BoxRect } from './useLabelStore'

export const MATCH_IOU = 0.5
export const TIGHT_IOU = 0.75

export interface ModelBox {
  labelTypeId: bigint
  attributes: Record<string, string>
  box?: BoxRect
  confidence: number
}

export interface VerifiedBox {
  labelTypeId: bigint
  attributes: Record<string, string>
  box: BoxRect
}

export interface Pair {
  model: number
  verified: number
  iou: number
  correct: boolean
  wrong?: string
}

export interface FrameMatch {
  pairs: Pair[]
  missed: number[]
  extra: number[]
}

// unchecked: the model found boxes on a frame nobody has marked done.
export type Verdict = 'correct' | 'off' | 'wrong' | 'unchecked'

export interface ClassStats {
  labelTypeId: bigint
  matched: number
  correct: number
  missed: number
  extra: number
}

export interface TrialReview {
  verdicts: (Verdict | undefined)[]
  classes: ClassStats[]
  all: Omit<ClassStats, 'labelTypeId'>
  labeledFrames: number
}

export function iou(a: BoxRect, b: BoxRect): number {
  const w = Math.min(a.x + a.width, b.x + b.width) - Math.max(a.x, b.x)
  const h = Math.min(a.y + a.height, b.y + b.height) - Math.max(a.y, b.y)
  if (w <= 0 || h <= 0) return 0
  const inter = w * h
  return inter / (a.width * a.height + b.width * b.height - inter)
}

// Confidences arrive as float32, so 0.35 can read as 0.3499999.
export function above<T extends ModelBox>(boxes: T[] | undefined, threshold: number): T[] {
  return boxes?.filter((b) => b.confidence >= threshold - 1e-6) ?? []
}

// Only attributes the verified track has set count, like gold checks.
function wrongAttribute(verified: VerifiedBox, model: ModelBox): string | undefined {
  return Object.keys(verified.attributes).find(
    (n) => verified.attributes[n] && verified.attributes[n] !== model.attributes[n],
  )
}

export function matchFrame(model: ModelBox[], verified: VerifiedBox[]): FrameMatch {
  const candidates: { model: number; verified: number; iou: number }[] = []
  model.forEach((m, i) => {
    if (!m.box) return
    verified.forEach((v, j) => {
      if (m.labelTypeId !== v.labelTypeId) return
      const o = iou(m.box!, v.box)
      if (o >= MATCH_IOU) candidates.push({ model: i, verified: j, iou: o })
    })
  })
  candidates.sort((a, b) => b.iou - a.iou)
  const usedModel = new Set<number>()
  const usedVerified = new Set<number>()
  const pairs: Pair[] = []
  for (const c of candidates) {
    if (usedModel.has(c.model) || usedVerified.has(c.verified)) continue
    usedModel.add(c.model)
    usedVerified.add(c.verified)
    const wrong = wrongAttribute(verified[c.verified], model[c.model])
    pairs.push({ ...c, correct: c.iou >= TIGHT_IOU && !wrong, wrong })
  }
  return {
    pairs,
    missed: verified.map((_, j) => j).filter((j) => !usedVerified.has(j)),
    extra: model.map((_, i) => i).filter((i) => !usedModel.has(i)),
  }
}

export function verdict(m: FrameMatch): Verdict {
  if (m.missed.length > 0 || m.extra.length > 0) return 'wrong'
  return m.pairs.every((p) => p.correct) ? 'correct' : 'off'
}

export function isDone(status: FrameStatus | undefined): boolean {
  return status === FrameStatus.LABELED || status === FrameStatus.EMPTY
}

export function verifiedBoxes(
  byTrack: Record<string, Annotation> | undefined,
  tracks: Record<string, Track>,
): VerifiedBox[] {
  const out: VerifiedBox[] = []
  for (const [k, a] of Object.entries(byTrack ?? {})) {
    const t = tracks[k]
    if (a.status !== AnnotationStatus.VERIFIED || !t || !a.box) continue
    out.push({ labelTypeId: t.labelTypeId, attributes: t.attributes, box: a.box })
  }
  return out
}

// verifiedOn returns undefined for frames not marked done. processed is how
// many frames from the start of the clip the trial has finished.
export function reviewTrial(
  frames: { id: bigint }[],
  processed: number,
  boxes: Record<string, ModelBox[]>,
  threshold: number,
  verifiedOn: (frameId: bigint) => VerifiedBox[] | undefined,
): TrialReview {
  const byType = new Map<bigint, ClassStats>()
  const stat = (id: bigint) => {
    let s = byType.get(id)
    if (!s) byType.set(id, (s = { labelTypeId: id, matched: 0, correct: 0, missed: 0, extra: 0 }))
    return s
  }
  let labeledFrames = 0
  const verdicts = frames.map((f, i): Verdict | undefined => {
    if (i >= processed) return undefined
    const model = above(boxes[String(f.id)], threshold)
    const verified = verifiedOn(f.id)
    if (!verified) return model.length > 0 ? 'unchecked' : undefined
    labeledFrames++
    const m = matchFrame(model, verified)
    for (const p of m.pairs) {
      const s = stat(verified[p.verified].labelTypeId)
      s.matched++
      if (p.correct) s.correct++
    }
    for (const j of m.missed) stat(verified[j].labelTypeId).missed++
    for (const k of m.extra) stat(model[k].labelTypeId).extra++
    return verdict(m)
  })
  const classes = [...byType.values()]
  const all = { matched: 0, correct: 0, missed: 0, extra: 0 }
  for (const c of classes) {
    all.matched += c.matched
    all.correct += c.correct
    all.missed += c.missed
    all.extra += c.extra
  }
  return { verdicts, classes, all, labeledFrames }
}

// Wraps past the end, and lands on the current frame only if it's the one left.
export function nextDisagreement(verdicts: (Verdict | undefined)[], from: number): number {
  const n = verdicts.length
  for (let d = 1; d <= n; d++) {
    const i = (from + d) % n
    if (verdicts[i] === 'off' || verdicts[i] === 'wrong') return i
  }
  return -1
}

export function precision(s: { matched: number; extra: number }): number | undefined {
  return s.matched + s.extra > 0 ? s.matched / (s.matched + s.extra) : undefined
}

export function recall(s: { matched: number; missed: number }): number | undefined {
  return s.matched + s.missed > 0 ? s.matched / (s.matched + s.missed) : undefined
}
