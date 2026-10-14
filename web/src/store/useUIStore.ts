import { create } from 'zustand'
import { persist } from 'zustand/middleware'

export type FlashType = 'success' | 'info' | 'error'

export interface FlashMessage {
  id: string
  type: FlashType
  title: string
  message: string
}

interface UIState {
  flashes: FlashMessage[]
  addFlash: (flash: Omit<FlashMessage, 'id'>, timeoutMs?: number) => void
  removeFlash: (id: string) => void
  displayCorrection: boolean
  toggleDisplayCorrection: () => void
}

export const useUIStore = create<UIState>()(
  persist(
    (set, get) => ({
      flashes: [],
      addFlash: (flash, timeoutMs = 5000) => {
        // A held hotkey would otherwise stack copies of the same toast.
        const shown = get().flashes.some(
          (f) => f.type === flash.type && f.title === flash.title && f.message === flash.message,
        )
        if (shown) return
        const id = crypto.randomUUID()
        set((s) => ({ flashes: [...s.flashes, { ...flash, id }] }))
        setTimeout(() => get().removeFlash(id), timeoutMs)
      },
      removeFlash: (id) => set((s) => ({ flashes: s.flashes.filter((f) => f.id !== id) })),
      displayCorrection: false,
      toggleDisplayCorrection: () => set((s) => ({ displayCorrection: !s.displayCorrection })),
    }),
    { name: 'krill-ui', partialize: (s) => ({ displayCorrection: s.displayCorrection }) },
  ),
)
