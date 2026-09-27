import type { GoldBox, GoldResult } from '@/gen/krill/v1/gold_pb'
import type { LabelType } from '@/gen/krill/v1/label_pb'

export type Outcome = 'correct' | 'off' | 'missed' | 'extra'

export interface ResultBox {
  box: GoldBox
  answer: boolean
  outcome: Outcome
  caption: string
}

export const outcomeColors: Record<Outcome, string> = {
  correct: '#10b981',
  off: '#f59e0b',
  missed: '#f43f5e',
  extra: '#f43f5e',
}

function wrongAttribute(ref: GoldBox, ans: GoldBox): string | undefined {
  const names = new Set([...Object.keys(ref.attributes), ...Object.keys(ans.attributes)])
  return [...names].find((n) => ref.attributes[n] !== ans.attributes[n])
}

export function resultBoxes(result: GoldResult, types: LabelType[]): ResultBox[] {
  const name = (b: GoldBox) => types.find((t) => t.id === b.labelTypeId)?.name ?? 'unknown'
  const byRef = new Map(result.matches.map((m) => [m.reference, m]))
  const matchedAnswers = new Set(result.matches.map((m) => m.answer))

  const out: ResultBox[] = result.reference.map((box, i) => {
    const m = byRef.get(i)
    if (!m) return { box, answer: false, outcome: 'missed', caption: `${name(box)} · missed` }
    if (m.correct) return { box, answer: false, outcome: 'correct', caption: name(box) }
    const attr = wrongAttribute(box, result.answer[m.answer])
    const why = attr ? `wrong ${attr}` : `IoU ${m.iou.toFixed(2)}`
    return { box, answer: false, outcome: 'off', caption: `${name(box)} · ${why}` }
  })
  result.answer.forEach((box, i) => {
    if (!matchedAnswers.has(i))
      out.push({ box, answer: true, outcome: 'extra', caption: `${name(box)} · extra` })
  })
  return out
}
