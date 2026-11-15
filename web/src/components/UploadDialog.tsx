import { XMarkIcon } from '@heroicons/react/20/solid'
import { useState } from 'react'
import { DropZone } from '@/components/DropZone'
import { Button } from '@/components/ui/Button'
import {
  Dialog,
  DialogActions,
  DialogBody,
  DialogDescription,
  DialogTitle,
} from '@/components/ui/Dialog'
import { Field, FieldGroup, Label } from '@/components/ui/Fieldset'
import { Input } from '@/components/ui/Input'
import { Select } from '@/components/ui/Select'
import { VideoService } from '@/gen/krill/v1/video_pb'
import { videoClient } from '@/lib/clients'
import { formatBytes } from '@/lib/format'
import { invalidateService } from '@/lib/queryClient'
import { putParts } from '@/lib/upload'
import { extractFpsOptions } from '@/lib/video'
import { useUploadStore, type UploadTask } from '@/store/useUploadStore'

interface Pending {
  file: File
  name: string
}

const DEFAULT_FPS = 10

function stripExtension(filename: string): string {
  const dot = filename.lastIndexOf('.')
  return dot > 0 ? filename.slice(0, dot) : filename
}

function videoUpload(file: File, name: string, extractFps: number): UploadTask {
  return {
    name,
    size: file.size,
    run: async (signal, onProgress) => {
      let videoId: bigint | undefined
      try {
        const created = await videoClient.createVideo({
          name,
          filename: file.name,
          extractFps,
          size: BigInt(file.size),
        })
        videoId = created.video?.id
        await invalidateService(VideoService)
        await putParts(created.partUrls, Number(created.partSize), file, onProgress, signal)
        await videoClient.startIngest({ videoId, uploadId: created.uploadId })
        return { to: `/videos/${videoId}`, label: 'View video' }
      } catch (err) {
        if (signal.aborted && videoId !== undefined) {
          await videoClient.deleteVideo({ id: videoId }).catch(() => undefined)
        }
        throw err
      } finally {
        await invalidateService(VideoService)
      }
    },
  }
}

export function UploadDialog({ open, onClose }: { open: boolean; onClose: () => void }) {
  const start = useUploadStore((s) => s.start)
  const [pending, setPending] = useState<Pending[]>([])
  const [fps, setFps] = useState(DEFAULT_FPS)

  function addFiles(files: File[]) {
    const videos = files.filter(
      (f) => f.type.startsWith('video/') || /\.(mp4|mov|mkv|avi|webm)$/i.test(f.name),
    )
    setPending((p) => [...p, ...videos.map((file) => ({ file, name: stripExtension(file.name) }))])
  }

  function close() {
    setPending([])
    setFps(DEFAULT_FPS)
    onClose()
  }

  function submit(e: React.FormEvent) {
    e.preventDefault()
    if (pending.length === 0) return
    start(pending.map((p) => videoUpload(p.file, p.name, fps)))
    close()
  }

  const totalBytes = pending.reduce((sum, p) => sum + p.file.size, 0)

  return (
    <Dialog open={open} onClose={close} size="xl">
      <form onSubmit={submit}>
        <DialogTitle>Upload videos</DialogTitle>
        <DialogDescription>Uploads keep going if you leave this page.</DialogDescription>
        <DialogBody>
          <FieldGroup>
            <DropZone
              label="videos"
              hint="MP4, MOV, MKV, or WebM"
              accept="video/*"
              onFiles={addFiles}
            />

            {pending.length > 0 && (
              <ul className="divide-y divide-zinc-950/5 rounded-lg ring-1 ring-zinc-950/10 dark:divide-white/5 dark:ring-white/10">
                {pending.map((p, i) => (
                  <li key={`${p.file.name}-${i}`} className="flex items-center gap-3 px-3 py-2">
                    <Input
                      aria-label={`Name for ${p.file.name}`}
                      value={p.name}
                      onChange={(e) =>
                        setPending((all) =>
                          all.map((x, j) => (j === i ? { ...x, name: e.target.value } : x)),
                        )
                      }
                    />
                    <span className="shrink-0 text-xs text-zinc-500 tabular-nums dark:text-zinc-400">
                      {formatBytes(p.file.size)}
                    </span>
                    <Button
                      plain
                      aria-label={`Remove ${p.file.name}`}
                      onClick={() => setPending((all) => all.filter((_, j) => j !== i))}
                    >
                      <XMarkIcon data-slot="icon" />
                    </Button>
                  </li>
                ))}
              </ul>
            )}

            <Field>
              <Label>Extraction rate</Label>
              <Select value={fps} onChange={(e) => setFps(Number(e.target.value))}>
                {extractFpsOptions.map((o) => (
                  <option key={o.value} value={o.value}>
                    {o.label}
                  </option>
                ))}
              </Select>
            </Field>
          </FieldGroup>
        </DialogBody>
        <DialogActions>
          <Button plain onClick={close}>
            Cancel
          </Button>
          <Button
            type="submit"
            color="sky"
            disabled={pending.length === 0 || pending.some((p) => !p.name.trim())}
          >
            {pending.length > 1
              ? `Upload ${pending.length} videos (${formatBytes(totalBytes)})`
              : 'Upload'}
          </Button>
        </DialogActions>
      </form>
    </Dialog>
  )
}
