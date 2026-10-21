import { describe, expect, it } from 'vitest'
import {
  matchFrame,
  nextDisagreement,
  reviewTrial,
  verdict,
  type ModelBox,
  type VerifiedBox,
} from './trialMatch'

const BIN = 1n
const HOLE = 2n

function model(
  type: bigint,
  x: number,
  attributes: Record<string, string> = {},
  confidence = 0.9,
): ModelBox {
  return { labelTypeId: type, attributes, box: { x, y: 0.1, width: 0.2, height: 0.2 }, confidence }
}

function verified(type: bigint, x: number, attributes: Record<string, string> = {}): VerifiedBox {
  return { labelTypeId: type, attributes, box: { x, y: 0.1, width: 0.2, height: 0.2 } }
}

describe('matchFrame', () => {
  it('ignores attributes the verified track leaves unset', () => {
    const m = matchFrame(
      [model(HOLE, 0.1, { size: 'big', color: 'red' })],
      [verified(HOLE, 0.1, { size: 'big' })],
    )
    expect(m.pairs).toHaveLength(1)
    expect(m.pairs[0].correct).toBe(true)
  })

  it('marks a tight box with a wrong set attribute off', () => {
    const m = matchFrame(
      [model(HOLE, 0.1, { size: 'small' })],
      [verified(HOLE, 0.1, { size: 'big' })],
    )
    expect(m.pairs[0]).toMatchObject({ correct: false, wrong: 'size' })
    expect(verdict(m)).toBe('off')
  })

  it('marks a model box missing a set attribute off', () => {
    const m = matchFrame([model(HOLE, 0.1)], [verified(HOLE, 0.1, { size: 'big' })])
    expect(m.pairs[0].wrong).toBe('size')
  })

  it('matches a loose box but does not call it correct', () => {
    // IoU of two 0.2 boxes offset by 0.03 is about 0.74.
    const m = matchFrame([model(BIN, 0.13)], [verified(BIN, 0.1)])
    expect(m.pairs).toHaveLength(1)
    expect(m.pairs[0].correct).toBe(false)
  })

  it('pairs greedily by best overlap, not input order', () => {
    // The first model box overlaps both verified boxes, but the second one
    // overlaps the first verified box better, so it takes it.
    const m = matchFrame(
      [model(BIN, 0.15), model(BIN, 0.1)],
      [verified(BIN, 0.1), verified(BIN, 0.2)],
    )
    expect(m.pairs.map((p) => [p.model, p.verified])).toEqual(
      expect.arrayContaining([
        [1, 0],
        [0, 1],
      ]),
    )
    expect(m.missed).toEqual([])
    expect(m.extra).toEqual([])
  })

  it('never pairs boxes of different types', () => {
    const m = matchFrame([model(HOLE, 0.1)], [verified(BIN, 0.1)])
    expect(m.pairs).toEqual([])
    expect(m.missed).toEqual([0])
    expect(m.extra).toEqual([0])
    expect(verdict(m)).toBe('wrong')
  })

  it('leaves a box under the match overlap unpaired', () => {
    const m = matchFrame(
      [model(BIN, 0.1), model(BIN, 0.6)],
      [verified(BIN, 0.1), verified(BIN, 0.3)],
    )
    expect(m.pairs.map((p) => [p.model, p.verified])).toEqual([[0, 0]])
    expect(m.missed).toEqual([1])
    expect(m.extra).toEqual([1])
  })
})

describe('reviewTrial', () => {
  const frames = [1n, 2n, 3n, 4n].map((id) => ({ id }))
  const boxes = {
    '1': [model(BIN, 0.1)],
    '2': [model(BIN, 0.1, {}, 0.3)],
    '3': [model(BIN, 0.1)],
    '4': [model(BIN, 0.1)],
  }
  const done: Record<string, VerifiedBox[]> = {
    '1': [verified(BIN, 0.1)],
    '2': [verified(BIN, 0.1)],
  }
  const on = (id: bigint) => done[String(id)]

  it('drops boxes under the threshold before matching', () => {
    const r = reviewTrial(frames, 4, boxes, 0.5, on)
    expect(r.verdicts.slice(0, 2)).toEqual(['correct', 'wrong'])
    expect(r.all).toEqual({ matched: 1, correct: 1, missed: 1, extra: 0 })
    expect(reviewTrial(frames, 4, boxes, 0.25, on).all.missed).toBe(0)
  })

  it('only scores frames marked done that the trial has reached', () => {
    const r = reviewTrial(frames, 3, boxes, 0.5, on)
    expect(r.verdicts).toEqual(['correct', 'wrong', 'unchecked', undefined])
    expect(r.labeledFrames).toBe(2)
  })
})

describe('nextDisagreement', () => {
  it('wraps past the end and skips agreeing frames', () => {
    const v = ['off', 'correct', 'unchecked', undefined, 'correct'] as const
    expect(nextDisagreement([...v], 1)).toBe(0)
    expect(nextDisagreement(['correct', 'unchecked', undefined], 0)).toBe(-1)
  })
})
