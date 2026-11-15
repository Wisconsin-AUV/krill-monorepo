import { CloudArrowUpIcon } from '@heroicons/react/20/solid'
import { clsx } from 'clsx'
import { useRef, useState } from 'react'

export function DropZone({
  label,
  hint,
  accept,
  onFiles,
}: {
  label: string
  hint: string
  accept: string
  onFiles: (files: File[]) => void
}) {
  const [dragging, setDragging] = useState(false)
  const input = useRef<HTMLInputElement>(null)

  return (
    <>
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
          onFiles([...e.dataTransfer.files])
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
          Drop {label} here or <span className="text-sky-600 dark:text-sky-400">browse</span>
        </span>
        <span className="text-xs/5 text-zinc-500 dark:text-zinc-400">{hint}</span>
      </button>
      <input
        ref={input}
        type="file"
        accept={accept}
        multiple
        hidden
        onChange={(e) => {
          onFiles([...(e.target.files ?? [])])
          e.target.value = ''
        }}
      />
    </>
  )
}
