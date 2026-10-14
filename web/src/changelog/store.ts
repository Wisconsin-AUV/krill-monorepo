import { create } from 'zustand'
import { persist } from 'zustand/middleware'

interface ChangelogState {
  open: boolean
  // The release to scroll to when the dialog opens.
  focus: string | null
  // The newest release this browser has been shown.
  seen: string | null
  show: (focus?: string) => void
  close: () => void
  markSeen: (version: string) => void
}

export const useChangelog = create<ChangelogState>()(
  persist(
    (set) => ({
      open: false,
      focus: null,
      seen: null,
      show: (focus) => set({ open: true, focus: focus ?? null }),
      close: () => set({ open: false }),
      markSeen: (version) => set({ seen: version }),
    }),
    { name: 'krill-changelog', partialize: (s) => ({ seen: s.seen }) },
  ),
)
