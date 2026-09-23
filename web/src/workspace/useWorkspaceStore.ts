import { create } from 'zustand'
import type { Frame } from '@/gen/krill/v1/clip_pb'

export interface View {
  scale: number
  x: number
  y: number
}

export interface Size {
  width: number
  height: number
}

interface WorkspaceState {
  frames: Frame[]
  index: number
  playing: boolean
  view: View
  stageSize: Size
  imageSize: Size
  setFrames: (frames: Frame[], index: number) => void
  seek: (index: number) => void
  step: (delta: number) => void
  setPlaying: (playing: boolean) => void
  setView: (view: View) => void
  setStageSize: (size: Size) => void
  setImageSize: (size: Size) => void
  fit: () => void
  zoomAt: (factor: number, point?: { x: number; y: number }) => void
}

const MIN_ZOOM = 0.5
const MAX_ZOOM = 20

export function fitView(stage: Size, image: Size): View {
  if (!stage.width || !stage.height || !image.width || !image.height)
    return { scale: 1, x: 0, y: 0 }
  const scale = Math.min(stage.width / image.width, stage.height / image.height)
  return {
    scale,
    x: (stage.width - image.width * scale) / 2,
    y: (stage.height - image.height * scale) / 2,
  }
}

export const useWorkspaceStore = create<WorkspaceState>((set, get) => ({
  frames: [],
  index: 0,
  playing: false,
  view: { scale: 1, x: 0, y: 0 },
  stageSize: { width: 0, height: 0 },
  imageSize: { width: 0, height: 0 },
  setFrames: (frames, index) =>
    set({
      frames,
      index: Math.min(Math.max(index, 0), Math.max(frames.length - 1, 0)),
      playing: false,
    }),
  seek: (index) => {
    const last = get().frames.length - 1
    set({ index: Math.min(Math.max(Math.round(index), 0), Math.max(last, 0)) })
  },
  step: (delta) => get().seek(get().index + delta),
  setPlaying: (playing) => set({ playing }),
  setView: (view) => set({ view }),
  setStageSize: (stageSize) => {
    const { imageSize, view, stageSize: prev } = get()
    const wasFit = sameView(view, fitView(prev, imageSize))
    set({ stageSize, ...(wasFit ? { view: fitView(stageSize, imageSize) } : {}) })
  },
  setImageSize: (imageSize) => {
    const prev = get().imageSize
    if (prev.width === imageSize.width && prev.height === imageSize.height) return
    set({ imageSize, view: fitView(get().stageSize, imageSize) })
  },
  fit: () => set({ view: fitView(get().stageSize, get().imageSize) }),
  zoomAt: (factor, point) => {
    const { view, stageSize, imageSize } = get()
    const base = fitView(stageSize, imageSize).scale
    const scale = Math.min(Math.max(view.scale * factor, base * MIN_ZOOM), base * MAX_ZOOM)
    const p = point ?? { x: stageSize.width / 2, y: stageSize.height / 2 }
    const ix = (p.x - view.x) / view.scale
    const iy = (p.y - view.y) / view.scale
    set({ view: { scale, x: p.x - ix * scale, y: p.y - iy * scale } })
  },
}))

function sameView(a: View, b: View): boolean {
  return (
    Math.abs(a.scale - b.scale) < 1e-6 && Math.abs(a.x - b.x) < 0.5 && Math.abs(a.y - b.y) < 0.5
  )
}
