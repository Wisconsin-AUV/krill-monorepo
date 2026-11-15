import { XMarkIcon } from '@heroicons/react/20/solid'
import { useState } from 'react'
import { DropZone } from '@/components/DropZone'
import { Button } from '@/components/ui/Button'
import { Dialog, DialogActions, DialogBody, DialogTitle } from '@/components/ui/Dialog'
import { DistractorService } from '@/gen/krill/v1/distractor_pb'
import { distractorClient } from '@/lib/clients'
import { formatBytes, plural } from '@/lib/format'
import { invalidateService } from '@/lib/queryClient'
import { putFile } from '@/lib/upload'
import { useUploadStore, type UploadTask } from '@/store/useUploadStore'

const types = ['image/jpeg', 'image/png', 'image/webp']

function distractorUpload(file: File): UploadTask {
  return {
    name: file.name,
    size: file.size,
    run: async (signal, onProgress) => {
      let id: bigint | undefined
      let processing = false
      try {
        const created = await distractorClient.createDistractor({
          filename: file.name,
          size: BigInt(file.size),
        })
        id = created.distractor?.id
        await invalidateService(DistractorService)
        await putFile(created.uploadUrl, file, onProgress, signal)
        await distractorClient.processDistractor({ id })
        processing = true
        return { to: '/admin/distractors', label: 'View distractors' }
      } catch (err) {
        // Unlike a video, there's nothing to see of a distractor that never
        // finished uploading.
        if (!processing && id !== undefined) {
          await distractorClient.deleteDistractors({ ids: [id] }).catch(() => undefined)
        }
        throw err
      } finally {
        await invalidateService(DistractorService)
      }
    },
  }
}

export function DistractorUploadDialog({ open, onClose }: { open: boolean; onClose: () => void }) {
  const start = useUploadStore((s) => s.start)
  const [pending, setPending] = useState<File[]>([])

  function close() {
    setPending([])
    onClose()
  }

  const totalBytes = pending.reduce((sum, f) => sum + f.size, 0)

  return (
    <Dialog open={open} onClose={close} size="xl">
      <form
        onSubmit={(e) => {
          e.preventDefault()
          if (pending.length === 0) return
          start(pending.map(distractorUpload))
          close()
        }}
      >
        <DialogTitle>Upload distractors</DialogTitle>
        <DialogBody className="space-y-6">
          <DropZone
            label="images"
            hint="JPEG, PNG, or WebP"
            accept={types.join(',')}
            onFiles={(files) =>
              setPending((p) => [...p, ...files.filter((f) => types.includes(f.type))])
            }
          />
          {pending.length > 0 && (
            <ul className="max-h-72 divide-y divide-zinc-950/5 overflow-y-auto rounded-lg ring-1 ring-zinc-950/10 dark:divide-white/5 dark:ring-white/10">
              {pending.map((f, i) => (
                <li key={`${f.name}-${i}`} className="flex items-center gap-3 py-1 pr-1 pl-3">
                  <span className="min-w-0 flex-1 truncate text-sm/6 text-zinc-950 dark:text-white">
                    {f.name}
                  </span>
                  <span className="shrink-0 text-xs text-zinc-500 tabular-nums dark:text-zinc-400">
                    {formatBytes(f.size)}
                  </span>
                  <Button
                    plain
                    aria-label={`Remove ${f.name}`}
                    onClick={() => setPending((all) => all.filter((_, j) => j !== i))}
                  >
                    <XMarkIcon data-slot="icon" />
                  </Button>
                </li>
              ))}
            </ul>
          )}
        </DialogBody>
        <DialogActions>
          <Button plain onClick={close}>
            Cancel
          </Button>
          <Button type="submit" color="sky" disabled={pending.length === 0}>
            {pending.length > 1
              ? `Upload ${plural(pending.length, 'image')} (${formatBytes(totalBytes)})`
              : 'Upload'}
          </Button>
        </DialogActions>
      </form>
    </Dialog>
  )
}
