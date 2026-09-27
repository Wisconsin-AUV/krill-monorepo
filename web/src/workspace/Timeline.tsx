import { ChevronLeftIcon, ChevronRightIcon, PauseIcon, PlayIcon } from '@heroicons/react/20/solid'
import { clsx } from 'clsx'
import { useRef, useState } from 'react'
import { Button } from '@/components/ui/Button'
import { formatFrameTime } from '@/lib/format'
import { useWorkspaceStore } from './useWorkspaceStore'

// Past this many frames the gaps between segments blur into noise.
const MAX_SEGMENT_GAPS = 150

export function Timeline({
  frameClass,
  trackClass,
  flagged,
}: {
  frameClass?: (position: number) => string | undefined
  trackClass?: (position: number) => string | undefined
  flagged?: (position: number) => boolean
}) {
  const frames = useWorkspaceStore((s) => s.frames)
  const index = useWorkspaceStore((s) => s.index)
  const playing = useWorkspaceStore((s) => s.playing)
  const seek = useWorkspaceStore((s) => s.seek)
  const step = useWorkspaceStore((s) => s.step)
  const setPlaying = useWorkspaceStore((s) => s.setPlaying)
  const track = useRef<HTMLDivElement>(null)
  const [hover, setHover] = useState<number | null>(null)
  const n = frames.length

  function positionAt(clientX: number): number {
    const rect = track.current?.getBoundingClientRect()
    if (!rect || n === 0) return 0
    return Math.min(Math.max(Math.floor(((clientX - rect.left) / rect.width) * n), 0), n - 1)
  }

  if (n === 0) return null
  const last = frames[n - 1]
  const current = frames[index]
  const center = (i: number) => `${((i + 0.5) / n) * 100}%`

  return (
    <div className="flex items-center gap-3 border-t border-zinc-950/10 bg-white px-3 py-2 select-none dark:border-white/10 dark:bg-zinc-900">
      <div className="flex items-center">
        <Button plain title="Previous frame (J)" disabled={index === 0} onClick={() => step(-1)}>
          <ChevronLeftIcon data-slot="icon" />
        </Button>
        <Button plain title="Play or pause (P)" onClick={() => setPlaying(!playing)}>
          {playing ? <PauseIcon data-slot="icon" /> : <PlayIcon data-slot="icon" />}
        </Button>
        <Button plain title="Next frame (K)" disabled={index === n - 1} onClick={() => step(1)}>
          <ChevronRightIcon data-slot="icon" />
        </Button>
      </div>

      <div
        ref={track}
        className="group relative h-10 flex-1 cursor-pointer touch-none"
        onPointerDown={(e) => {
          e.currentTarget.setPointerCapture(e.pointerId)
          setPlaying(false)
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
        <div
          className={clsx(
            'absolute inset-x-0 top-2 flex h-5 overflow-hidden rounded-md',
            n <= MAX_SEGMENT_GAPS && 'gap-px',
          )}
        >
          {frames.map((f, i) => (
            <div
              key={String(f.id)}
              className={clsx(
                'h-full flex-1 transition-opacity',
                frameClass?.(i) || 'bg-zinc-200 dark:bg-zinc-800',
                hover === i && 'opacity-70',
              )}
            />
          ))}
        </div>
        {flagged && (
          <div className="absolute inset-x-0 top-0.5 flex h-1">
            {frames.map((f, i) => (
              <div
                key={String(f.id)}
                className={clsx('h-full flex-1 rounded-full', flagged(i) && 'bg-rose-500')}
              />
            ))}
          </div>
        )}
        {trackClass && (
          <div className="absolute inset-x-0 bottom-1 flex h-1 overflow-hidden rounded-full">
            {frames.map((f, i) => (
              <div key={String(f.id)} className={clsx('h-full flex-1', trackClass(i))} />
            ))}
          </div>
        )}

        {hover !== null && hover !== index && (
          <div
            className="pointer-events-none absolute bottom-full z-10 mb-2 -translate-x-1/2 overflow-hidden rounded-md bg-zinc-900 shadow-lg ring-1 ring-white/10"
            style={{ left: center(hover) }}
          >
            <img src={frames[hover].url} alt="" className="block w-40" />
            <div className="px-2 py-1 text-center text-[11px] whitespace-nowrap text-zinc-300 tabular-nums">
              {formatFrameTime(frames[hover].timestampMs)} · frame {hover + 1}
            </div>
          </div>
        )}

        <div
          className="pointer-events-none absolute inset-y-0 w-0.5 -translate-x-1/2 bg-sky-500 dark:bg-sky-400"
          style={{ left: center(index) }}
        >
          <div className="absolute -top-1 left-1/2 size-3 -translate-x-1/2 rounded-full bg-sky-500 ring-2 ring-white dark:bg-sky-400 dark:ring-zinc-900" />
        </div>
      </div>

      <span className="w-28 text-right text-xs/5 text-zinc-500 tabular-nums dark:text-zinc-400">
        <span className="text-zinc-950 dark:text-white">
          {current && formatFrameTime(current.timestampMs)}
        </span>{' '}
        / {formatFrameTime(last.timestampMs)}
      </span>
    </div>
  )
}
