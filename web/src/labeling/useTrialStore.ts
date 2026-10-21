import { create } from 'zustand'
import type { ModelTrial, ModelTrialBox } from '@/gen/krill/v1/pipeline_pb'

export type TrialMode = 'boxes' | 'compare'

export const MIN_THRESHOLD = 0.1
export const MAX_THRESHOLD = 0.95
const DEFAULT_THRESHOLD = 0.5

interface TrialState {
  trial: ModelTrial | null
  // frame id -> boxes
  boxes: Record<string, ModelTrialBox[]>
  threshold: number
  mode: TrialMode
  visible: boolean

  load: (trial: ModelTrial, boxes: ModelTrialBox[]) => void
  clear: () => void
  setThreshold: (t: number) => void
  setMode: (m: TrialMode) => void
  toggleVisible: () => void
}

function byFrame(boxes: ModelTrialBox[]): Record<string, ModelTrialBox[]> {
  const out: Record<string, ModelTrialBox[]> = {}
  for (const b of boxes) (out[String(b.frameId)] ??= []).push(b)
  return out
}

const clamp = (t: number) => Math.min(Math.max(t, MIN_THRESHOLD), MAX_THRESHOLD)

export const useTrialStore = create<TrialState>((set, get) => ({
  trial: null,
  boxes: {},
  threshold: DEFAULT_THRESHOLD,
  mode: 'boxes',
  visible: true,

  load: (trial, boxes) => {
    const fresh = get().trial?.id !== trial.id
    set({
      trial,
      boxes: byFrame(boxes),
      ...(fresh && {
        threshold: clamp(trial.summary?.minConfidence || DEFAULT_THRESHOLD),
        visible: true,
      }),
    })
  },
  clear: () => set({ trial: null, boxes: {} }),
  setThreshold: (threshold) => set({ threshold: clamp(threshold) }),
  setMode: (mode) => set({ mode }),
  toggleVisible: () => set((s) => ({ visible: !s.visible })),
}))
