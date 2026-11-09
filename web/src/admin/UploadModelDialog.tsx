import { CloudArrowUpIcon } from '@heroicons/react/20/solid'
import { clsx } from 'clsx'
import { useEffect, useRef, useState } from 'react'
import { useNavigate } from 'react-router'
import { Meter } from '@/components/Meter'
import { Button } from '@/components/ui/Button'
import { FormError } from '@/components/ui/Callout'
import { Dialog, DialogActions, DialogBody, DialogTitle } from '@/components/ui/Dialog'
import { PipelineService } from '@/gen/krill/v1/pipeline_pb'
import { pipelineClient } from '@/lib/clients'
import { formatBytes } from '@/lib/format'
import { invalidateService } from '@/lib/queryClient'
import { putFile } from '@/lib/upload'

type Step = { kind: 'idle' } | { kind: 'uploading'; loaded: number } | { kind: 'checking' }

function UploadForm({ onClose }: { onClose: () => void }) {
  const navigate = useNavigate()
  const [file, setFile] = useState<File | null>(null)
  const [step, setStep] = useState<Step>({ kind: 'idle' })
  const [error, setError] = useState<unknown>(null)
  const [dragging, setDragging] = useState(false)
  const input = useRef<HTMLInputElement>(null)
  const abort = useRef<AbortController | null>(null)
  const busy = step.kind !== 'idle'
  useEffect(() => () => abort.current?.abort(), [])

  function pick(list: FileList | null) {
    const f = list?.[0]
    if (!f) return
    setFile(f)
    setError(null)
  }

  async function submit(e: React.FormEvent) {
    e.preventDefault()
    if (!file) return
    const ctrl = new AbortController()
    abort.current = ctrl
    setError(null)
    setStep({ kind: 'uploading', loaded: 0 })
    try {
      const { key, uploadUrl } = await pipelineClient.createModelUpload({})
      await putFile(
        uploadUrl,
        file,
        (loaded) => setStep({ kind: 'uploading', loaded }),
        ctrl.signal,
      )
      setStep({ kind: 'checking' })
      const { run } = await pipelineClient.addModel(
        { key, filename: file.name },
        { signal: ctrl.signal },
      )
      await invalidateService(PipelineService)
      onClose()
      if (run) navigate(`/admin/models/${run.id}`)
    } catch (err) {
      if (ctrl.signal.aborted) return
      setError(err)
      setStep({ kind: 'idle' })
    }
  }

  return (
    <form onSubmit={(e) => void submit(e)}>
      <DialogTitle>Upload a model</DialogTitle>
      <DialogBody>
        <button
          type="button"
          disabled={busy}
          onClick={() => input.current?.click()}
          onDragOver={(e) => {
            e.preventDefault()
            setDragging(true)
          }}
          onDragLeave={() => setDragging(false)}
          onDrop={(e) => {
            e.preventDefault()
            setDragging(false)
            if (!busy) pick(e.dataTransfer.files)
          }}
          className={clsx(
            'flex w-full flex-col items-center rounded-xl border border-dashed px-6 py-10 text-center transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-sky-500',
            dragging
              ? 'border-sky-500 bg-sky-500/5'
              : 'border-zinc-950/15 hover:border-zinc-950/30 dark:border-white/15 dark:hover:border-white/30',
          )}
        >
          <CloudArrowUpIcon className="size-10 text-zinc-400" aria-hidden="true" />
          {file ? (
            <>
              <span className="mt-3 text-sm/6 font-semibold break-all text-zinc-950 dark:text-white">
                {file.name}
              </span>
              <span className="text-xs/5 text-zinc-500 tabular-nums dark:text-zinc-400">
                {formatBytes(file.size)}
              </span>
            </>
          ) : (
            <>
              <span className="mt-3 text-sm/6 font-semibold text-zinc-950 dark:text-white">
                Drop weights here or <span className="text-sky-600 dark:text-sky-400">browse</span>
              </span>
              <span className="text-xs/5 text-zinc-500 dark:text-zinc-400">
                Ultralytics YOLO detection .pt
              </span>
            </>
          )}
        </button>
        <input
          ref={input}
          type="file"
          accept=".pt"
          hidden
          onChange={(e) => {
            pick(e.target.files)
            e.target.value = ''
          }}
        />
        {step.kind === 'uploading' && file && (
          <Meter label="Uploading" value={step.loaded / file.size} className="mt-4" />
        )}
        <FormError error={error} className="mt-4" />
      </DialogBody>
      <DialogActions>
        <Button plain onClick={onClose}>
          Cancel
        </Button>
        <Button type="submit" color="sky" disabled={!file || busy}>
          {step.kind === 'uploading'
            ? 'Uploading…'
            : step.kind === 'checking'
              ? 'Reading classes…'
              : 'Upload'}
        </Button>
      </DialogActions>
    </form>
  )
}

export function UploadModelDialog({ open, onClose }: { open: boolean; onClose: () => void }) {
  return (
    <Dialog open={open} onClose={onClose} size="xl">
      {open && <UploadForm onClose={onClose} />}
    </Dialog>
  )
}
