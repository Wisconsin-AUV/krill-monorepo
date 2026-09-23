import { create } from 'zustand'
import { VideoService } from '@/gen/krill/v1/video_pb'
import { videoClient } from '@/lib/clients'
import { errorMessage } from '@/lib/errors'
import { invalidateService } from '@/lib/queryClient'
import { putFile } from '@/lib/upload'

export type UploadState = 'uploading' | 'queued' | 'error' | 'canceled'

export interface Upload {
  id: string
  name: string
  size: number
  loaded: number
  state: UploadState
  error?: string
  videoId?: bigint
}

interface UploadStore {
  uploads: Upload[]
  start: (files: { file: File; name: string }[], extractFps: number) => void
  cancel: (id: string) => void
  dismiss: (id: string) => void
  clearFinished: () => void
}

const controllers = new Map<string, AbortController>()

export const useUploadStore = create<UploadStore>((set, get) => {
  const patch = (id: string, p: Partial<Upload>) =>
    set((s) => ({ uploads: s.uploads.map((u) => (u.id === id ? { ...u, ...p } : u)) }))

  async function run(id: string, file: File, name: string, extractFps: number) {
    const controller = new AbortController()
    controllers.set(id, controller)
    let videoId: bigint | undefined
    try {
      const created = await videoClient.createVideo({ name, filename: file.name, extractFps })
      videoId = created.video?.id
      patch(id, { videoId })
      await invalidateService(VideoService)
      await putFile(created.uploadUrl, file, (loaded) => patch(id, { loaded }), controller.signal)
      await videoClient.startIngest({ videoId })
      patch(id, { state: 'queued', loaded: file.size })
    } catch (err) {
      const canceled = controller.signal.aborted
      patch(id, { state: canceled ? 'canceled' : 'error', error: errorMessage(err) })
      if (canceled && videoId !== undefined) {
        await videoClient.deleteVideo({ id: videoId }).catch(() => undefined)
      }
    } finally {
      controllers.delete(id)
      await invalidateService(VideoService)
    }
  }

  return {
    uploads: [],
    start: (files, extractFps) => {
      const added = files.map(({ file, name }) => ({
        id: crypto.randomUUID(),
        name,
        size: file.size,
        loaded: 0,
        state: 'uploading' as const,
      }))
      set((s) => ({ uploads: [...s.uploads, ...added] }))
      added.forEach((u, i) => void run(u.id, files[i].file, u.name, extractFps))
    },
    cancel: (id) => controllers.get(id)?.abort(),
    dismiss: (id) => set((s) => ({ uploads: s.uploads.filter((u) => u.id !== id) })),
    clearFinished: () => set({ uploads: get().uploads.filter((u) => u.state === 'uploading') }),
  }
})
