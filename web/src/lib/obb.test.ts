import { describe, expect, it } from 'vitest'
import {
  bounds,
  fromCorners,
  fromTopLeft,
  isRotated,
  toCorners,
  topLeft,
  type Point,
  type RotatedRect,
} from './obb'

const wide = { width: 1920, height: 1080 }

function expectRect(actual: RotatedRect, expected: RotatedRect) {
  for (const k of ['cx', 'cy', 'width', 'height', 'rotation'] as const)
    expect(actual[k]).toBeCloseTo(expected[k], 6)
}

function expectPoints(actual: Point[], expected: Point[]) {
  expect(actual).toHaveLength(expected.length)
  actual.forEach((p, i) => {
    expect(p.x).toBeCloseTo(expected[i].x, 9)
    expect(p.y).toBeCloseTo(expected[i].y, 9)
  })
}

describe('obb', () => {
  it('round-trips a rotated rect on a non-square frame', () => {
    const r = { cx: 800, cy: 400, width: 300, height: 120, rotation: 30 }
    const corners = toCorners(r, wide)
    expectRect(fromCorners(corners, wide), r)
    expectPoints(toCorners(fromCorners(corners, wide), wide), corners)
  })

  it('keeps a rotated square square in pixels on a non-square frame', () => {
    const r = { cx: 960, cy: 540, width: 200, height: 200, rotation: 45 }
    const corners = toCorners(r, wide)
    expect(corners[0].y).toBeCloseTo(corners[2].y - (200 * Math.SQRT2) / 1080, 9)
    expect(corners[1].x).toBeCloseTo(corners[3].x + (200 * Math.SQRT2) / 1920, 9)
    expectRect(fromCorners(corners, wide), r)
    expect(isRotated(fromCorners(corners, wide))).toBe(true)
  })

  it('gives the same rect whichever corner the list starts from', () => {
    const r = { cx: 500, cy: 300, width: 240, height: 90, rotation: -20 }
    const corners = toCorners(r, wide)
    for (let k = 1; k < 4; k++) {
      const cycled = [...corners.slice(k), ...corners.slice(0, k)]
      expectRect(fromCorners(cycled, wide), r)
    }
  })

  it('treats a quarter turn as unrotated with swapped sides', () => {
    const corners = toCorners({ cx: 100, cy: 100, width: 80, height: 40, rotation: 90 }, wide)
    const r = fromCorners(corners, wide)
    expectRect(r, { cx: 100, cy: 100, width: 40, height: 80, rotation: 0 })
    expect(isRotated(r)).toBe(false)
  })

  it('converts between Konva top-left nodes and centers', () => {
    const r = { cx: 640, cy: 360, width: 160, height: 60, rotation: 37 }
    const tl = topLeft(r)
    expectRect(fromTopLeft({ ...tl, width: 160, height: 60, rotation: 37 }), r)
  })

  it('normalizes a Konva node turned past 45 degrees to the same shape', () => {
    const node = { x: 300, y: 200, width: 160, height: 60, rotation: 127 }
    const corners = toCorners(fromTopLeft(node), wide)
    const r = fromCorners(corners, wide)
    expect(r.rotation).toBeCloseTo(37, 6)
    expect(r.width).toBeCloseTo(60, 6)
    expect(r.height).toBeCloseTo(160, 6)
    const again = toCorners(r, wide)
    const same = again.every((p) =>
      corners.some((q) => Math.abs(p.x - q.x) < 1e-9 && Math.abs(p.y - q.y) < 1e-9),
    )
    expect(same).toBe(true)
  })

  it('clips bounds to the frame', () => {
    const b = bounds([
      { x: -0.1, y: 0.2 },
      { x: 0.3, y: -0.05 },
      { x: 0.5, y: 0.4 },
      { x: 0.1, y: 1.2 },
    ])
    expect(b.x).toBe(0)
    expect(b.y).toBe(0)
    expect(b.width).toBeCloseTo(0.5)
    expect(b.height).toBe(1)
  })
})
