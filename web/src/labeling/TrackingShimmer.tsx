import { useState } from 'react'
import type { Size, View } from '@/workspace/useWorkspaceStore'
import type { BoxRect, TrackPrompt } from './useLabelStore'

const POINT_SIZE_PX = 96
const MIN_CELL_PX = 14
const MAX_CELLS = 400
const WAVE_SPREAD_S = 0.8

function screenRect(prompt: TrackPrompt, box: BoxRect | undefined, view: View, image: Size) {
  const rect = box ?? ('box' in prompt ? prompt.box : undefined)
  if (!rect) {
    const { x, y } = 'point' in prompt ? prompt.point : { x: 0, y: 0 }
    return {
      left: view.x + x * image.width * view.scale - POINT_SIZE_PX / 2,
      top: view.y + y * image.height * view.scale - POINT_SIZE_PX / 2,
      width: POINT_SIZE_PX,
      height: POINT_SIZE_PX,
    }
  }
  return {
    left: view.x + rect.x * image.width * view.scale,
    top: view.y + rect.y * image.height * view.scale,
    width: rect.width * image.width * view.scale,
    height: rect.height * image.height * view.scale,
  }
}

// Deterministic per seed and cell, so the pattern holds still across renders.
function hash(seed: number, a: number, b: number): number {
  let h = seed ^ Math.imul(a, 0x27d4eb2d) ^ Math.imul(b, 0x165667b1)
  h = Math.imul(h ^ (h >>> 15), 0x85ebca6b)
  h = Math.imul(h ^ (h >>> 13), 0xc2b2ae35)
  return ((h ^ (h >>> 16)) >>> 0) / 4294967296
}

export function TrackingShimmer({
  prompt,
  box,
  view,
  imageSize,
}: {
  prompt: TrackPrompt
  box?: BoxRect
  view: View
  imageSize: Size
}) {
  const [seed] = useState(() => Math.floor(Math.random() * 2 ** 31))
  const rect = screenRect(prompt, box, view, imageSize)
  const cell = Math.max(MIN_CELL_PX, Math.sqrt((rect.width * rect.height) / MAX_CELLS))
  const cols = Math.max(Math.floor(rect.width / cell), 1)
  const rows = Math.max(Math.floor(rect.height / cell), 1)
  const clumps = Array.from({ length: 3 }, (_, i) => ({
    x: hash(seed, i, -1) * cols,
    y: hash(seed, i, -2) * rows,
  }))
  const radius = Math.max(Math.min(cols, rows) / 2.5, 1.5)

  const dots = []
  for (let r = 0; r < rows; r++) {
    for (let c = 0; c < cols; c++) {
      const near = Math.max(
        ...clumps.map((k) => Math.exp(-((c - k.x) ** 2 + (r - k.y) ** 2) / radius ** 2)),
      )
      if (hash(seed, c, r) > near * 0.9) continue
      const size = cell * (0.3 + 0.5 * hash(seed, r, c))
      dots.push(
        <span
          key={`${c}-${r}`}
          className="animate-dot-wave absolute rounded-[30%] bg-zinc-100/20"
          style={{
            left: (rect.width - cols * cell) / 2 + (c + 0.5) * cell - size / 2,
            top: (rect.height - rows * cell) / 2 + (r + 0.5) * cell - size / 2,
            width: size,
            height: size,
            animationDelay: `${(c / cols) * WAVE_SPREAD_S}s`,
          }}
        />,
      )
    }
  }

  return (
    <div className="pointer-events-none absolute" style={rect}>
      {dots}
    </div>
  )
}
