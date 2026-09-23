import { clsx } from 'clsx'
import { useRef, useState } from 'react'
import { formatFrameTime } from '@/lib/format'
import { useWorkspaceStore } from './useWorkspaceStore'

export function Timeline({
  frameClass,
  trackClass,
}: {
  frameClass?: (position: number) => string | undefined
  trackClass?: (position: number) => string | undefined
}) {
  const frames = useWorkspaceStore((s) => s.frames)
  const index = useWorkspaceStore((s) => s.index)
  const seek = useWorkspaceStore((s) => s.seek)
  const track = useRef<HTMLDivElement>(null)
  const [hover, setHover] = useState<number | null>(null)
  const n = frames.length

  function positionAt(clientX: number): number {
    const rect = track.current?.getBoundingClientRect()
    if (!rect || n === 0) return 0
    return Math.min(Math.max(Math.floor(((clientX - rect.left) / rect.width) * n), 0), n - 1)
  }

  if (n === 0) return null
  const first = frames[0]
  const last = frames[n - 1]
  const current = frames[index]

  return (
    <div className="border-t border-zinc-950/10 bg-white px-4 pt-2 pb-3 select-none dark:border-white/10 dark:bg-zinc-900">
      <div className="mb-1.5 flex justify-between text-xs/5 text-zinc-500 tabular-nums dark:text-zinc-400">
        <span>{formatFrameTime(first.timestampMs)}</span>
        <span className="text-zinc-800 dark:text-zinc-200">
          {current && `${formatFrameTime(current.timestampMs)} · frame ${current.index}`}
        </span>
        <span>{formatFrameTime(last.timestampMs)}</span>
      </div>
      <div
        ref={track}
        className="relative h-8 cursor-pointer touch-none"
        onPointerDown={(e) => {
          e.currentTarget.setPointerCapture(e.pointerId)
          seek(positionAt(e.clientX))
        }}
        onPointerMove={(e) => {
          setHover(positionAt(e.clientX))
          if (e.buttons === 1) seek(positionAt(e.clientX))
        }}
        onPointerLeave={() => setHover(null)}
      >
        <input
          type="range"
          aria-label="Frame"
          min={0}
          max={n - 1}
          value={index}
          onChange={(e) => seek(Number(e.target.value))}
          tabIndex={-1}
          className="sr-only"
        />
        <div className="absolute inset-y-2 left-0 flex w-full overflow-hidden rounded bg-zinc-200 dark:bg-zinc-800">
          {frameClass &&
            frames.map((f, i) => (
              <div key={String(f.id)} className={clsx('h-full flex-1', frameClass(i))} />
            ))}
        </div>
        {trackClass && (
          <div className="absolute bottom-0 left-0 flex h-1 w-full overflow-hidden rounded-full">
            {frames.map((f, i) => (
              <div key={String(f.id)} className={clsx('h-full flex-1', trackClass(i))} />
            ))}
          </div>
        )}
        {hover !== null && hover !== index && (
          <div
            className="pointer-events-none absolute inset-y-1 w-px bg-zinc-950/30 dark:bg-white/30"
            style={{ left: `${((hover + 0.5) / n) * 100}%` }}
          >
            <span className="absolute -top-6 -translate-x-1/2 rounded bg-zinc-700 px-1.5 py-0.5 text-[11px] whitespace-nowrap text-zinc-950 tabular-nums dark:text-white">
              {frames[hover].index}
            </span>
          </div>
        )}
        <div
          className="pointer-events-none absolute inset-y-0 w-0.5 -translate-x-1/2 rounded-full bg-sky-500 shadow-[0_0_0_2px] shadow-white dark:bg-sky-400 dark:shadow-zinc-900"
          style={{ left: `${((index + 0.5) / n) * 100}%` }}
        />
      </div>
    </div>
  )
}
