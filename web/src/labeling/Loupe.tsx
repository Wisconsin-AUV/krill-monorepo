import { useEffect, useRef } from 'react'
import type { Size, View } from '@/workspace/useWorkspaceStore'

const SIZE_PX = 132
const ZOOM = 4
const OFFSET_PX = 28
const GAP_PX = 5

// Magnifies the frame around a point in image pixels, beside the cursor.
export function Loupe({
  image,
  at,
  view,
  stageSize,
}: {
  image: CanvasImageSource
  at: { x: number; y: number }
  view: View
  stageSize: Size
}) {
  const canvas = useRef<HTMLCanvasElement>(null)
  const dpr = window.devicePixelRatio || 1

  useEffect(() => {
    const ctx = canvas.current?.getContext('2d')
    if (!ctx) return
    const span = SIZE_PX / (view.scale * ZOOM)
    const mid = SIZE_PX / 2
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0)
    ctx.fillStyle = '#09090b'
    ctx.fillRect(0, 0, SIZE_PX, SIZE_PX)
    ctx.imageSmoothingEnabled = false
    ctx.drawImage(image, at.x - span / 2, at.y - span / 2, span, span, 0, 0, SIZE_PX, SIZE_PX)
    for (const [color, width] of [
      ['#09090b', 3],
      ['#ffffff', 1],
    ] as const) {
      ctx.strokeStyle = color
      ctx.lineWidth = width
      ctx.beginPath()
      ctx.moveTo(0, mid)
      ctx.lineTo(mid - GAP_PX, mid)
      ctx.moveTo(mid + GAP_PX, mid)
      ctx.lineTo(SIZE_PX, mid)
      ctx.moveTo(mid, 0)
      ctx.lineTo(mid, mid - GAP_PX)
      ctx.moveTo(mid, mid + GAP_PX)
      ctx.lineTo(mid, SIZE_PX)
      ctx.stroke()
    }
  }, [image, at.x, at.y, view.scale, dpr])

  const sx = view.x + at.x * view.scale
  const sy = view.y + at.y * view.scale
  const left =
    sx + OFFSET_PX + SIZE_PX > stageSize.width ? sx - OFFSET_PX - SIZE_PX : sx + OFFSET_PX
  const top = sy - OFFSET_PX - SIZE_PX < 0 ? sy + OFFSET_PX : sy - OFFSET_PX - SIZE_PX
  return (
    <canvas
      ref={canvas}
      width={SIZE_PX * dpr}
      height={SIZE_PX * dpr}
      className="pointer-events-none absolute rounded-full shadow-lg ring-2 ring-white/70"
      style={{ left, top, width: SIZE_PX, height: SIZE_PX }}
    />
  )
}
