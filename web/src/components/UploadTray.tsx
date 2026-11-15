import { CheckCircleIcon, ExclamationCircleIcon, XMarkIcon } from '@heroicons/react/20/solid'
import { useEffect } from 'react'
import { Link } from 'react-router'
import { Meter } from '@/components/Meter'
import { formatBytes } from '@/lib/format'
import { active, useUploadStore, type Upload } from '@/store/useUploadStore'

function UploadRow({ upload }: { upload: Upload }) {
  const cancel = useUploadStore((s) => s.cancel)
  const dismiss = useUploadStore((s) => s.dismiss)
  const progress = upload.size > 0 ? upload.loaded / upload.size : 0

  return (
    <li className="px-4 py-3">
      <div className="flex items-center gap-2">
        {upload.state === 'done' && (
          <CheckCircleIcon className="size-4 shrink-0 text-green-500" aria-hidden="true" />
        )}
        {(upload.state === 'error' || upload.state === 'canceled') && (
          <ExclamationCircleIcon className="size-4 shrink-0 text-red-500" aria-hidden="true" />
        )}
        <span className="min-w-0 flex-1 truncate text-sm/6 font-medium text-zinc-950 dark:text-white">
          {upload.name}
        </span>
        <button
          type="button"
          onClick={() => (active(upload) ? cancel(upload.id) : dismiss(upload.id))}
          className="rounded p-0.5 text-zinc-400 hover:text-zinc-600 focus-visible:outline-2 focus-visible:outline-sky-500 dark:hover:text-zinc-200"
          aria-label={active(upload) ? `Cancel ${upload.name}` : `Dismiss ${upload.name}`}
        >
          <XMarkIcon className="size-4" />
        </button>
      </div>
      {upload.state === 'uploading' && (
        <>
          <Meter value={progress} label={`Uploading ${upload.name}`} className="mt-2" />
          <p className="mt-1 text-xs/5 text-zinc-500 tabular-nums dark:text-zinc-400">
            {formatBytes(upload.loaded)} of {formatBytes(upload.size)}
          </p>
        </>
      )}
      {upload.state === 'waiting' && (
        <p className="text-xs/5 text-zinc-500 dark:text-zinc-400">Waiting</p>
      )}
      {upload.state === 'done' && (
        <p className="text-xs/5 text-zinc-500 dark:text-zinc-400">
          Uploaded.{' '}
          {upload.link && (
            <Link
              to={upload.link.to}
              className="font-medium text-sky-600 hover:text-sky-500 dark:text-sky-400"
            >
              {upload.link.label}
            </Link>
          )}
        </p>
      )}
      {upload.state === 'canceled' && (
        <p className="text-xs/5 text-zinc-500 dark:text-zinc-400">Canceled</p>
      )}
      {upload.state === 'error' && (
        <p className="text-xs/5 text-red-600 dark:text-red-400">{upload.error}</p>
      )}
    </li>
  )
}

export function UploadTray() {
  const uploads = useUploadStore((s) => s.uploads)
  const clearFinished = useUploadStore((s) => s.clearFinished)
  const running = uploads.filter(active).length

  useEffect(() => {
    if (running === 0) return
    const warn = (e: BeforeUnloadEvent) => e.preventDefault()
    window.addEventListener('beforeunload', warn)
    return () => window.removeEventListener('beforeunload', warn)
  }, [running])

  if (uploads.length === 0) return null

  return (
    <section
      aria-label="Uploads"
      className="fixed right-4 bottom-4 z-40 w-80 overflow-hidden rounded-xl bg-white shadow-lg ring-1 ring-zinc-950/10 dark:bg-zinc-900 dark:ring-white/10"
    >
      <header className="flex items-center justify-between border-b border-zinc-950/5 px-4 py-2.5 dark:border-white/5">
        <h2 className="text-sm/6 font-semibold text-zinc-950 dark:text-white">
          {running > 0 ? `Uploading ${running} of ${uploads.length}` : 'Uploads'}
        </h2>
        {running < uploads.length && (
          <button
            type="button"
            onClick={clearFinished}
            className="text-xs/5 font-medium text-zinc-500 hover:text-zinc-700 dark:text-zinc-400 dark:hover:text-zinc-200"
          >
            Clear finished
          </button>
        )}
      </header>
      <ul className="max-h-80 divide-y divide-zinc-950/5 overflow-y-auto dark:divide-white/5">
        {uploads.map((u) => (
          <UploadRow key={u.id} upload={u} />
        ))}
      </ul>
    </section>
  )
}
