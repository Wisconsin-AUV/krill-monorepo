import { create } from 'zustand'
import { errorMessage } from '@/lib/errors'

export type UploadState = 'waiting' | 'uploading' | 'done' | 'error' | 'canceled'

export interface UploadLink {
  to: string
  label: string
}

export interface Upload {
  id: string
  name: string
  size: number
  loaded: number
  state: UploadState
  error?: string
  link?: UploadLink
}

export interface UploadTask {
  name: string
  size: number
  // run cleans up whatever it made when signal aborts, and returns where the
  // finished upload can be seen.
  run: (signal: AbortSignal, onProgress: (loaded: number) => void) => Promise<UploadLink>
}

interface UploadStore {
  uploads: Upload[]
  start: (tasks: UploadTask[]) => void
  cancel: (id: string) => void
  dismiss: (id: string) => void
  clearFinished: () => void
}

// Video uploads already send several parts at once, and a big batch of
// images shouldn't open hundreds of requests.
const maxRunning = 3

const tasks = new Map<string, UploadTask>()
const controllers = new Map<string, AbortController>()

export function active(u: Upload) {
  return u.state === 'waiting' || u.state === 'uploading'
}

export const useUploadStore = create<UploadStore>((set, get) => {
  const patch = (id: string, p: Partial<Upload>) =>
    set((s) => ({ uploads: s.uploads.map((u) => (u.id === id ? { ...u, ...p } : u)) }))

  function pump() {
    let running = get().uploads.filter((u) => u.state === 'uploading').length
    for (const u of get().uploads) {
      if (running >= maxRunning) return
      if (u.state !== 'waiting') continue
      running++
      void run(u.id)
    }
  }

  async function run(id: string) {
    const task = tasks.get(id)
    if (!task) return
    tasks.delete(id)
    const controller = new AbortController()
    controllers.set(id, controller)
    patch(id, { state: 'uploading' })
    try {
      const link = await task.run(controller.signal, (loaded) => patch(id, { loaded }))
      patch(id, { state: 'done', loaded: task.size, link })
    } catch (err) {
      patch(id, {
        state: controller.signal.aborted ? 'canceled' : 'error',
        error: errorMessage(err),
      })
    } finally {
      controllers.delete(id)
      pump()
    }
  }

  return {
    uploads: [],
    start: (added) => {
      const uploads = added.map((t) => {
        const id = crypto.randomUUID()
        tasks.set(id, t)
        return { id, name: t.name, size: t.size, loaded: 0, state: 'waiting' as const }
      })
      set((s) => ({ uploads: [...s.uploads, ...uploads] }))
      pump()
    },
    cancel: (id) => {
      if (tasks.delete(id)) patch(id, { state: 'canceled' })
      else controllers.get(id)?.abort()
    },
    dismiss: (id) => set((s) => ({ uploads: s.uploads.filter((u) => u.id !== id) })),
    clearFinished: () => set({ uploads: get().uploads.filter(active) }),
  }
})
