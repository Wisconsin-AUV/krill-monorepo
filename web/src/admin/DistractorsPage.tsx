import { useMutation, useQuery } from '@connectrpc/connect-query'
import { ArrowUpTrayIcon, PhotoIcon, XMarkIcon } from '@heroicons/react/20/solid'
import { useEffect, useRef, useState } from 'react'
import { ConfirmAlert } from '@/components/ConfirmAlert'
import { EmptyState } from '@/components/EmptyState'
import { Meter } from '@/components/Meter'
import { Thumbnail } from '@/components/Thumbnail'
import { Button } from '@/components/ui/Button'
import { QueryError } from '@/components/ui/Callout'
import { Heading } from '@/components/ui/Heading'
import LoadingSpinner from '@/components/ui/LoadingSpinner'
import PageContentBlock from '@/components/ui/PageContentBlock'
import { DistractorService, type Distractor } from '@/gen/krill/v1/distractor_pb'
import { distractorClient } from '@/lib/clients'
import { flash } from '@/lib/flash'
import { plural } from '@/lib/format'
import { invalidateService } from '@/lib/queryClient'
import { putFile } from '@/lib/upload'

type Progress = { index: number; total: number; loaded: number; size: number }

export function DistractorsPage() {
  const { data, isPending, error } = useQuery(DistractorService.method.listDistractors, {})
  const [progress, setProgress] = useState<Progress | null>(null)
  const [deleting, setDeleting] = useState<Distractor | null>(null)
  const input = useRef<HTMLInputElement>(null)
  const abort = useRef<AbortController | null>(null)
  useEffect(() => () => abort.current?.abort(), [])
  const remove = useMutation(DistractorService.method.deleteDistractor, {
    onSuccess: async () => {
      await invalidateService(DistractorService)
      setDeleting(null)
    },
  })
  const distractors = data?.distractors ?? []

  async function upload(files: File[]) {
    const ctrl = new AbortController()
    abort.current = ctrl
    for (const [index, file] of files.entries()) {
      setProgress({ index, total: files.length, loaded: 0, size: file.size })
      try {
        const { key, uploadUrl } = await distractorClient.createDistractorUpload(
          {},
          { signal: ctrl.signal },
        )
        await putFile(
          uploadUrl,
          file,
          (loaded) => setProgress({ index, total: files.length, loaded, size: file.size }),
          ctrl.signal,
        )
        await distractorClient.addDistractor({ key, filename: file.name }, { signal: ctrl.signal })
      } catch (err) {
        if (ctrl.signal.aborted) return
        flash.error(`Could not upload ${file.name}`, err)
      }
    }
    setProgress(null)
    await invalidateService(DistractorService)
  }

  return (
    <PageContentBlock title="Distractors · Admin · Krill">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <Heading>Distractors</Heading>
        <div className="flex items-center gap-3">
          {data && (
            <span className="text-sm/6 text-zinc-500 tabular-nums dark:text-zinc-400">
              {plural(distractors.length, 'image')}
            </span>
          )}
          <Button color="sky" disabled={progress !== null} onClick={() => input.current?.click()}>
            <ArrowUpTrayIcon data-slot="icon" />
            {progress ? `Uploading ${progress.index + 1} of ${progress.total}…` : 'Upload'}
          </Button>
          <input
            ref={input}
            type="file"
            accept="image/png,image/jpeg,image/webp"
            multiple
            hidden
            onChange={(e) => {
              const files = [...(e.target.files ?? [])]
              e.target.value = ''
              if (files.length > 0) void upload(files)
            }}
          />
        </div>
      </div>
      {progress && (
        <Meter
          label="Uploading"
          value={
            (progress.index + (progress.size > 0 ? progress.loaded / progress.size : 0)) /
            progress.total
          }
          className="mt-4"
        />
      )}

      {isPending ? (
        <div className="flex justify-center py-16">
          <LoadingSpinner />
        </div>
      ) : error ? (
        <QueryError title="Could not load distractors" error={error} className="mt-8" />
      ) : distractors.length === 0 ? (
        <div className="mt-8">
          <EmptyState
            icon={PhotoIcon}
            title="No distractors"
            description="Scenes with none of the labeled objects."
          />
        </div>
      ) : (
        <ul className="mt-8 grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5">
          {distractors.map((d) => (
            <li key={String(d.id)} className="group relative min-w-0">
              <Thumbnail src={d.url} alt={d.filename} />
              <p className="mt-1.5 truncate text-sm/5 text-zinc-950 dark:text-white">
                {d.filename}
              </p>
              <p className="text-xs/5 text-zinc-500 tabular-nums dark:text-zinc-400">
                {d.width}×{d.height}
              </p>
              <button
                type="button"
                aria-label={`Delete ${d.filename}`}
                onClick={() => setDeleting(d)}
                className="absolute top-1 right-1 rounded-full bg-zinc-950/60 p-0.5 text-white opacity-0 group-hover:opacity-100 hover:bg-zinc-950/80 focus:opacity-100"
              >
                <XMarkIcon className="size-4" />
              </button>
            </li>
          ))}
        </ul>
      )}

      <ConfirmAlert
        open={deleting !== null}
        onClose={() => {
          setDeleting(null)
          remove.reset()
        }}
        onConfirm={() => deleting && remove.mutate({ id: deleting.id })}
        busy={remove.isPending}
        error={remove.error}
        title={`Delete ${deleting?.filename ?? ''}?`}
        description="The image is removed from storage."
        confirmLabel="Delete image"
      />
    </PageContentBlock>
  )
}
