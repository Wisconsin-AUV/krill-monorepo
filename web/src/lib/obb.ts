export interface Point {
  x: number
  y: number
}

interface Size {
  width: number
  height: number
}

// In pixels, rotated clockwise on screen about the center.
export interface RotatedRect {
  cx: number
  cy: number
  width: number
  height: number
  rotation: number
}

const RAD = Math.PI / 180
const EPSILON_DEG = 0.01

function pixelCorners(r: RotatedRect): Point[] {
  const c = Math.cos(r.rotation * RAD)
  const s = Math.sin(r.rotation * RAD)
  const hw = r.width / 2
  const hh = r.height / 2
  return [
    [-hw, -hh],
    [hw, -hh],
    [hw, hh],
    [-hw, hh],
  ].map(([dx, dy]) => ({ x: r.cx + dx * c - dy * s, y: r.cy + dx * s + dy * c }))
}

// Normalized corners, clockwise on screen from the unrotated top left.
export function toCorners(r: RotatedRect, size: Size): Point[] {
  return pixelCorners(r).map((p) => ({ x: p.x / size.width, y: p.y / size.height }))
}

// The rotation is kept within (-45, 45] by picking which side counts as the
// width, so handles and captions stay near the top whichever corner the
// corners start from.
export function fromCorners(corners: readonly Point[], size: Size): RotatedRect {
  const p = corners.map((c) => ({ x: c.x * size.width, y: c.y * size.height }))
  let width = Math.hypot(p[1].x - p[0].x, p[1].y - p[0].y)
  let height = Math.hypot(p[2].x - p[1].x, p[2].y - p[1].y)
  let rotation = Math.atan2(p[1].y - p[0].y, p[1].x - p[0].x) / RAD
  while (rotation > 45) {
    rotation -= 90
    ;[width, height] = [height, width]
  }
  while (rotation <= -45) {
    rotation += 90
    ;[width, height] = [height, width]
  }
  return {
    cx: (p[0].x + p[1].x + p[2].x + p[3].x) / 4,
    cy: (p[0].y + p[1].y + p[2].y + p[3].y) / 4,
    width,
    height,
    rotation,
  }
}

// Konva rotates a node about its x and y, which is the unrotated top left.
export function topLeft(r: RotatedRect): Point {
  return pixelCorners(r)[0]
}

export function fromTopLeft(n: {
  x: number
  y: number
  width: number
  height: number
  rotation: number
}): RotatedRect {
  const c = Math.cos(n.rotation * RAD)
  const s = Math.sin(n.rotation * RAD)
  return {
    cx: n.x + (n.width / 2) * c - (n.height / 2) * s,
    cy: n.y + (n.width / 2) * s + (n.height / 2) * c,
    width: n.width,
    height: n.height,
    rotation: n.rotation,
  }
}

export function isRotated(r: RotatedRect): boolean {
  const off = Math.abs(r.rotation % 90)
  return off > EPSILON_DEG && off < 90 - EPSILON_DEG
}

// The corners' normalized bounds clipped to the frame, as the server stores
// x, y, width, and height.
export function bounds(corners: readonly Point[]) {
  const xs = corners.map((p) => Math.min(Math.max(p.x, 0), 1))
  const ys = corners.map((p) => Math.min(Math.max(p.y, 0), 1))
  const x = Math.min(...xs)
  const y = Math.min(...ys)
  return { x, y, width: Math.max(...xs) - x, height: Math.max(...ys) - y }
}

// Konva Rect attributes in pixels. Corners count only for oriented types;
// other boxes are drawn from their bounds.
export function konvaBox(
  box: { x: number; y: number; width: number; height: number; corners?: readonly Point[] },
  oriented: boolean,
  size: Size,
) {
  if (oriented && box.corners?.length === 4) {
    const r = fromCorners(box.corners, size)
    return { ...topLeft(r), width: r.width, height: r.height, rotation: r.rotation }
  }
  return {
    x: box.x * size.width,
    y: box.y * size.height,
    width: box.width * size.width,
    height: box.height * size.height,
    rotation: 0,
  }
}
