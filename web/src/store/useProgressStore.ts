import { create } from 'zustand'

interface ProgressState {
  progress?: number
  continuous: boolean
  setProgress: (progress?: number) => void
  startContinuous: () => void
  setComplete: () => void
}

export const useProgressStore = create<ProgressState>((set) => ({
  progress: undefined,
  continuous: false,
  setProgress: (progress) => set({ progress }),
  startContinuous: () => set({ continuous: true }),
  setComplete: () => set((s) => ({ continuous: false, progress: s.progress ? 100 : undefined })),
}))
