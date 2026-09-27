import { create } from 'zustand'
import type { FlashMessageType } from '@/components/ui/Notification'

export interface FlashMessage {
  id: string
  key: string
  type: FlashMessageType
  title?: string
  message: string
}

interface UIState {
  flashes: FlashMessage[]
  addFlash: (flash: Omit<FlashMessage, 'id'>, timeoutMs?: number) => void
  clearFlashes: (key?: string) => void
}

export const useUIStore = create<UIState>((set) => ({
  flashes: [],
  addFlash: (flash, timeoutMs = 5000) => {
    const id = crypto.randomUUID()
    set((s) => ({ flashes: [...s.flashes, { ...flash, id }] }))
    if (timeoutMs > 0) {
      setTimeout(() => set((s) => ({ flashes: s.flashes.filter((f) => f.id !== id) })), timeoutMs)
    }
  },
  clearFlashes: (key) =>
    set((s) => ({ flashes: key ? s.flashes.filter((f) => f.key !== key) : [] })),
}))
