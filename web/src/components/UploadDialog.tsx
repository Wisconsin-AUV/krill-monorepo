import { CloudArrowUpIcon, XMarkIcon } from '@heroicons/react/20/solid'
import { clsx } from 'clsx'
import { useRef, useState } from 'react'
import { Button } from '@/components/ui/Button'
import {
  Dialog,
  DialogActions,
  DialogBody,
  DialogDescription,
  DialogTitle,
} from '@/components/ui/Dialog'
import { Description, Field, FieldGroup, Label } from '@/components/ui/Fieldset'
import { Input } from '@/components/ui/Input'
import { Select } from '@/components/ui/Select'
import { formatBytes } from '@/lib/format'
import { extractFpsOptions } from '@/lib/video'
import { useUploadStore } from '@/store/useUploadStore'

interface Pending {
  file: File
  name: string
}

const DEFAULT_FPS = 10

function stripExtension(filename: string): string {
  const dot = filename.lastIndexOf('.')
  return dot > 0 ? filename.slice(0, dot) : filename
}

export function UploadDialog({ open, onClose }: { open: boolean; onClose: () => void }) {
  const start = useUploadStore((s) => s.start)
  const [pending, setPending] = useState<Pending[]>([])
  const [fps, setFps] = useState(DEFAULT_FPS)
  const [dragging, setDragging] = useState(false)
  const input = useRef<HTMLInputElement>(null)

  function addFiles(list: FileList | null) {
    if (!list) return
    const videos = Array.from(list).filter(
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
    start(pending, fps)
    close()
  }

  const totalBytes = pending.reduce((sum, p) => sum + p.file.size, 0)

  return (
    <Dialog open={open} onClose={close} size="xl">
      <form onSubmit={submit}>
        <DialogTitle>Upload videos</DialogTitle>
        <DialogDescription>
          Frames are extracted and split into 10 to 20 second clips. Uploads keep going if you leave
          this page.
        </DialogDescription>
        <DialogBody>
          <FieldGroup>
            <button
              type="button"
              onClick={() => input.current?.click()}
              onDragOver={(e) => {
                e.preventDefault()
                setDragging(true)
              }}
              onDragLeave={() => setDragging(false)}
              onDrop={(e) => {
                e.preventDefault()
                setDragging(false)
                addFiles(e.dataTransfer.files)
              }}
              className={clsx(
                'flex w-full flex-col items-center rounded-xl border border-dashed px-6 py-10 text-center transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-sky-500',
                dragging
                  ? 'border-sky-500 bg-sky-500/5'
                  : 'border-zinc-950/15 hover:border-zinc-950/30 dark:border-white/15 dark:hover:border-white/30',
              )}
            >
              <CloudArrowUpIcon className="size-10 text-zinc-400" aria-hidden="true" />
              <span className="mt-3 text-sm/6 font-semibold text-zinc-950 dark:text-white">
                Drop videos here or <span className="text-sky-600 dark:text-sky-400">browse</span>
              </span>
              <span className="text-xs/5 text-zinc-500 dark:text-zinc-400">
                MP4, MOV, MKV, or WebM
              </span>
            </button>
            <input
              ref={input}
              type="file"
              accept="video/*"
              multiple
              hidden
              onChange={(e) => {
                addFiles(e.target.files)
                e.target.value = ''
              }}
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
              <Description>
                Lower rates save storage when the camera moves slowly. Export can stride further
                later.
              </Description>
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
