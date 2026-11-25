import { create } from '@bufbuild/protobuf'
import { describe, expect, it } from 'vitest'
import { KeypointVisibility } from '@/gen/krill/v1/annotation_pb'
import { LabelTypeSchema } from '@/gen/krill/v1/label_pb'
import {
  nearestUnplaced,
  nextUnlabeled,
  padKeypoints,
  promptQuad,
  quadCenter,
  setKeypoint,
  squareToQuad,
} from './keypoints'

const { UNSPECIFIED, HIDDEN, VISIBLE } = KeypointVisibility

// A sheet seen at an angle: the far (right) side is shorter.
const tilted = [
  { x: 0.1, y: 0.1 },
  { x: 0.6, y: 0.25 },
  { x: 0.6, y: 0.55 },
  { x: 0.1, y: 0.7 },
]

const sheet = create(LabelTypeSchema, {
  keypoints: ['tl', 'tr', 'br', 'bl', 'center', 'tip'].map((name) => ({ name })),
  quads: [{ corners: [0, 1, 2, 3], center: 4 }],
})

const at = (p: { x: number; y: number }, visibility: KeypointVisibility = VISIBLE) => ({
  ...p,
  visibility,
})

describe('keypoints', () => {
  it('puts the center where the diagonals cross, not at the corners mean', () => {
    const c = quadCenter(tilted)
    const mean = tilted.reduce((s, p) => ({ x: s.x + p.x / 4, y: s.y + p.y / 4 }), { x: 0, y: 0 })
    expect(Math.abs(c.x - mean.x)).toBeGreaterThan(0.01)
    // Both diagonals pass through it.
    for (const [a, b] of [
      [0, 2],
      [1, 3],
    ]) {
      const p = tilted[a]
      const q = tilted[b]
      expect((q.x - p.x) * (c.y - p.y) - (q.y - p.y) * (c.x - p.x)).toBeCloseTo(0, 12)
    }
  })

  it('maps the unit square onto the quad with its center on the diagonals', () => {
    const warp = squareToQuad(tilted)
    const corners = [
      [0, 0],
      [1, 0],
      [1, 1],
      [0, 1],
    ].map(([u, v]) => warp(u, v))
    corners.forEach((p, i) => {
      expect(p.x).toBeCloseTo(tilted[i].x, 12)
      expect(p.y).toBeCloseTo(tilted[i].y, 12)
    })
    const mid = warp(0.5, 0.5)
    const c = quadCenter(tilted)
    expect(mid.x).toBeCloseTo(c.x, 12)
    expect(mid.y).toBeCloseTo(c.y, 12)
  })

  it('pads keypoints saved before the type gained more', () => {
    const out = padKeypoints([at({ x: 0.2, y: 0.3 })], 3)
    expect(out.map((k) => k.visibility)).toEqual([VISIBLE, UNSPECIFIED, UNSPECIFIED])
  })

  it('derives the center once all four corners are labeled', () => {
    let kps = setKeypoint([], sheet, 0, at(tilted[0]))
    expect(kps).toHaveLength(6)
    kps = setKeypoint(kps, sheet, 1, at(tilted[1], HIDDEN))
    kps = setKeypoint(kps, sheet, 2, at(tilted[2]))
    expect(kps[4].visibility).toBe(UNSPECIFIED)
    kps = setKeypoint(kps, sheet, 3, at(tilted[3]))
    expect(kps[4].visibility).toBe(VISIBLE)
    expect(kps[4].x).toBeCloseTo(quadCenter(tilted).x, 12)

    kps = setKeypoint(kps, sheet, 2, at({ x: 0, y: 0 }, UNSPECIFIED))
    expect(kps[4].visibility).toBe(UNSPECIFIED)
  })

  it('hides the center only when every corner is hidden', () => {
    let kps = [0, 1, 2, 3].reduce(
      (acc, i) => setKeypoint(acc, sheet, i, at(tilted[i], HIDDEN)),
      [] as ReturnType<typeof setKeypoint>,
    )
    expect(kps[4].visibility).toBe(HIDDEN)
    kps = setKeypoint(kps, sheet, 0, at(tilted[0]))
    expect(kps[4].visibility).toBe(VISIBLE)
  })

  it('stores no keypoints once the last one is cleared', () => {
    const kps = setKeypoint([], sheet, 5, at({ x: 0.5, y: 0.5 }))
    expect(setKeypoint(kps, sheet, 5, at({ x: 0, y: 0 }, UNSPECIFIED))).toEqual([])
  })

  it('advances past labeled and derived keypoints, wrapping around', () => {
    let kps = setKeypoint([], sheet, 0, at(tilted[0]))
    kps = setKeypoint(kps, sheet, 5, at({ x: 0.5, y: 0.5 }))
    expect(nextUnlabeled(kps, sheet, 0)).toBe(1)
    kps = [1, 2, 3].reduce((acc, i) => setKeypoint(acc, sheet, i, at(tilted[i])), kps)
    expect(nextUnlabeled(kps, sheet, 3)).toBeNull()
    kps = setKeypoint(kps, sheet, 1, at({ x: 0, y: 0 }, UNSPECIFIED))
    expect(nextUnlabeled(kps, sheet, 3)).toBe(1)
  })

  it('fills the quad clicked inside before the one being placed', () => {
    const bin = create(LabelTypeSchema, {
      keypoints: ['p0', 'p1', 'p2', 'p3', 'r0', 'r1', 'r2', 'r3'].map((name) => ({ name })),
      quads: [{ corners: [0, 1, 2, 3], reference: true }, { corners: [4, 5, 6, 7] }],
    })
    const square = (x0: number, x1: number) =>
      [
        { x: x0, y: x0 },
        { x: x1, y: x0 },
        { x: x1, y: x1 },
        { x: x0, y: x1 },
      ].map((p) => at(p))
    // Nothing yet: the printed paper, though the rim's corner is the one up next.
    expect(promptQuad(bin, [], 4, { x: 0.5, y: 0.5 })).toBe(0)
    // A paper filled wrong is filled again when clicked inside, not the rim.
    const paper = square(0.4, 0.6)
    expect(promptQuad(bin, paper, 4, { x: 0.5, y: 0.5 })).toBe(0)
    // Off the paper, the rim being placed.
    expect(promptQuad(bin, paper, 4, { x: 0.2, y: 0.2 })).toBe(1)
    // Inside both, the smaller one.
    const both = [...paper, ...square(0.1, 0.9)]
    expect(promptQuad(bin, both, null, { x: 0.5, y: 0.5 })).toBe(0)
    expect(promptQuad(bin, both, null, { x: 0.2, y: 0.2 })).toBe(1)
  })

  it('guesses the unplaced corner a click is for from the placed ones', () => {
    const placed = (...corners: ({ x: number; y: number } | null)[]) =>
      padKeypoints(
        corners.map((p) => (p ? at(p) : at({ x: 0, y: 0 }, UNSPECIFIED))),
        sheet.keypoints.length,
      )
    const nearest = (kps: ReturnType<typeof placed>, x: number, y: number, aspect = 1) =>
      nearestUnplaced(kps, sheet, { x, y }, aspect)

    // A wall hides the bottom edge: which bottom corner is by the side of it.
    const top = placed({ x: 0.2, y: 0.2 }, { x: 0.6, y: 0.2 })
    expect(nearest(top, 0.55, 0.5)).toBe(2)
    expect(nearest(top, 0.25, 0.45)).toBe(3)
    // Placed across a diagonal, by the side of it.
    const diagonal = placed({ x: 0.2, y: 0.2 }, null, { x: 0.6, y: 0.6 })
    expect(nearest(diagonal, 0.5, 0.3)).toBe(1)
    expect(nearest(diagonal, 0.3, 0.5)).toBe(3)
    // The last corner wherever the click is.
    const three = placed({ x: 0.2, y: 0.2 }, { x: 0.6, y: 0.2 }, { x: 0.6, y: 0.6 })
    expect(nearest(three, 0.9, 0.1)).toBe(3)
    // One corner doesn't say where the rest are.
    expect(nearest(placed({ x: 0.2, y: 0.2 }), 0.6, 0.6)).toBeNull()
    expect(nearest(placed(), 0.6, 0.6)).toBeNull()
    // The square guessed is one in pixels: the top edge here is steeper on
    // a wide frame, which turns the bottom corners' sides.
    const slanted = placed({ x: 0.2, y: 0.2 }, { x: 0.4, y: 0.4 })
    expect(nearest(slanted, 0.15, 0.55)).toBe(2)
    expect(nearest(slanted, 0.15, 0.55, 2)).toBe(3)
  })
})
