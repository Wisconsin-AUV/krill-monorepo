import { useEffect, useMemo, useState } from 'react'
import { Image as KonvaImage } from 'react-konva'
import type { LabelType } from '@/gen/krill/v1/label_pb'
import { isLabeled, padKeypoints, squareToQuad, type KeypointValue } from '@/lib/keypoints'
import type { Point } from '@/lib/obb'
import type { Size } from '@/workspace/useWorkspaceStore'

const GRID = 8
const OPACITY = 0.35
const MAX_SIDE_PX = 4096
// Cells overlap by this much so their antialiased edges don't leave seams.
const BLEED_PX = 0.6

const images = new Map<string, HTMLImageElement>()

function useImage(url: string): HTMLImageElement | undefined {
  const [, setLoaded] = useState(0)
  const img = useMemo(() => {
    if (!url) return undefined
    let hit = images.get(url)
    if (!hit) {
      hit = new Image()
      hit.src = url
      images.set(url, hit)
    }
    return hit
  }, [url])
  useEffect(() => {
    if (!img || img.complete) return
    const done = () => setLoaded((n) => n + 1)
    img.addEventListener('load', done)
    return () => img.removeEventListener('load', done)
  }, [img])
  return img?.complete && img.naturalWidth > 0 ? img : undefined
}

// Affine transform taking triangle s onto triangle d.
function affine(s: Point[], d: Point[]): [number, number, number, number, number, number] {
  const s1 = { x: s[1].x - s[0].x, y: s[1].y - s[0].y }
  const s2 = { x: s[2].x - s[0].x, y: s[2].y - s[0].y }
  const d1 = { x: d[1].x - d[0].x, y: d[1].y - d[0].y }
  const d2 = { x: d[2].x - d[0].x, y: d[2].y - d[0].y }
  const det = s1.x * s2.y - s2.x * s1.y
  const a = (d1.x * s2.y - d2.x * s1.y) / det
  const c = (s1.x * d2.x - s2.x * d1.x) / det
  const b = (d1.y * s2.y - d2.y * s1.y) / det
  const dd = (s1.x * d2.y - s2.x * d1.y) / det
  return [a, b, c, dd, d[0].x - a * s[0].x - c * s[0].y, d[0].y - b * s[0].x - dd * s[0].y]
}

function grow(t: Point[]): Point[] {
  const cx = (t[0].x + t[1].x + t[2].x) / 3
  const cy = (t[0].y + t[1].y + t[2].y) / 3
  return t.map((p) => {
    const len = Math.hypot(p.x - cx, p.y - cy) || 1
    return { x: p.x + ((p.x - cx) / len) * BLEED_PX, y: p.y + ((p.y - cy) / len) * BLEED_PX }
  })
}

// Perspective warp drawn as a grid of affine triangles, which canvas can do.
// Drawn opaque offscreen so the overlap between cells doesn't show once
// faded.
function warp(img: HTMLImageElement, corners: Point[]) {
  const xs = corners.map((p) => p.x)
  const ys = corners.map((p) => p.y)
  const x = Math.floor(Math.min(...xs))
  const y = Math.floor(Math.min(...ys))
  const width = Math.ceil(Math.max(...xs)) - x
  const height = Math.ceil(Math.max(...ys)) - y
  if (width < 2 || height < 2 || width > MAX_SIDE_PX || height > MAX_SIDE_PX) return undefined
  const canvas = document.createElement('canvas')
  canvas.width = width
  canvas.height = height
  const ctx = canvas.getContext('2d')
  if (!ctx) return undefined
  const map = squareToQuad(corners.map((p) => ({ x: p.x - x, y: p.y - y })))
  const iw = img.naturalWidth
  const ih = img.naturalHeight
  const draw = (s: Point[], d: Point[]) => {
    const g = grow(d)
    ctx.save()
    ctx.beginPath()
    ctx.moveTo(g[0].x, g[0].y)
    ctx.lineTo(g[1].x, g[1].y)
    ctx.lineTo(g[2].x, g[2].y)
    ctx.closePath()
    ctx.clip()
    ctx.transform(...affine(s, d))
    ctx.drawImage(img, 0, 0)
    ctx.restore()
  }
  for (let i = 0; i < GRID; i++) {
    for (let j = 0; j < GRID; j++) {
      const [u0, u1, v0, v1] = [i / GRID, (i + 1) / GRID, j / GRID, (j + 1) / GRID]
      const src = (u: number, v: number) => ({ x: u * iw, y: v * ih })
      const s = [src(u0, v0), src(u1, v0), src(u1, v1), src(u0, v1)]
      const d = [map(u0, v0), map(u1, v0), map(u1, v1), map(u0, v1)]
      draw([s[0], s[1], s[2]], [d[0], d[1], d[2]])
      draw([s[0], s[2], s[3]], [d[0], d[2], d[3]])
    }
  }
  return { canvas, x, y }
}

// The type's reference design laid over each reference quad of a box, so a
// quad whose corners are in the wrong order shows the design turned.
export function ReferenceOverlay({
  type,
  keypoints,
  imageSize,
}: {
  type: LabelType
  keypoints: readonly KeypointValue[]
  imageSize: Size
}) {
  const img = useImage(type.referenceUrl)
  const quads = useMemo(() => {
    if (!img) return []
    const kps = padKeypoints(keypoints, type.keypoints.length)
    return type.quads.flatMap((q) => {
      if (!q.reference || q.corners.length !== 4) return []
      const corners = q.corners.map((i) => kps[i])
      if (!corners.every(isLabeled)) return []
      const w = warp(
        img,
        corners.map((k) => ({ x: k.x * imageSize.width, y: k.y * imageSize.height })),
      )
      return w ? [w] : []
    })
  }, [img, keypoints, type, imageSize])

  return quads.map((q, i) => (
    <KonvaImage key={i} image={q.canvas} x={q.x} y={q.y} opacity={OPACITY} listening={false} />
  ))
}
