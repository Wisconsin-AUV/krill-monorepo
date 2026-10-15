import { useState } from 'react'
import type { EpochMetrics } from '@/gen/krill/v1/pipeline_pb'
import { percent } from './pipeline'

const W = 640
const H = 160
const PAD = 4

const series = [
  { key: 'map50', label: 'mAP50', className: 'stroke-sky-300 dark:stroke-sky-700' },
  { key: 'map5095', label: 'mAP50-95', className: 'stroke-sky-500' },
] as const

// Validation scores after each epoch, out of the run's total so the line
// grows toward the right edge as training goes.
export function EpochChart({ history, epochs }: { history: EpochMetrics[]; epochs: number }) {
  const [hover, setHover] = useState<number | null>(null)
  const n = Math.max(epochs, history.length, 2)
  const x = (epoch: number) => PAD + ((epoch - 1) / (n - 1)) * (W - 2 * PAD)
  const y = (v: number) => H - PAD - Math.min(Math.max(v, 0), 1) * (H - 2 * PAD)
  const shown = hover !== null ? history[hover] : history.at(-1)

  return (
    <div>
      <div className="flex flex-wrap items-baseline gap-x-6 gap-y-1 text-sm/6">
        {shown && (
          <span className="text-zinc-500 tabular-nums dark:text-zinc-400">Epoch {shown.epoch}</span>
        )}
        {series.map((s) => (
          <span key={s.key} className="flex items-center gap-2">
            <span className={`h-0.5 w-4 ${s.className.replaceAll('stroke-', 'bg-')}`} />
            <span className="text-zinc-500 dark:text-zinc-400">{s.label}</span>
            <span className="font-medium text-zinc-950 tabular-nums dark:text-white">
              {shown ? percent(shown[s.key]) : '—'}
            </span>
          </span>
        ))}
      </div>
      <svg
        viewBox={`0 0 ${W} ${H}`}
        className="mt-3 h-40 w-full rounded-md bg-zinc-950/2.5 dark:bg-white/2.5"
        preserveAspectRatio="none"
        onPointerLeave={() => setHover(null)}
        onPointerMove={(e) => {
          if (history.length === 0) return
          const r = e.currentTarget.getBoundingClientRect()
          const epoch = 1 + Math.round(((e.clientX - r.left) / r.width) * (n - 1))
          setHover(Math.min(Math.max(epoch, 1), history.length) - 1)
        }}
      >
        <title>Validation score by epoch</title>
        {series.map((s) => (
          <polyline
            key={s.key}
            fill="none"
            strokeWidth={2}
            vectorEffect="non-scaling-stroke"
            className={s.className}
            points={history.map((h) => `${x(h.epoch)},${y(h[s.key])}`).join(' ')}
          />
        ))}
        {hover !== null && history[hover] && (
          <line
            x1={x(history[hover].epoch)}
            x2={x(history[hover].epoch)}
            y1={0}
            y2={H}
            vectorEffect="non-scaling-stroke"
            className="stroke-zinc-400"
          />
        )}
      </svg>
    </div>
  )
}
