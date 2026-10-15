import { useId, useState } from 'react'

function ago(seconds: number) {
  return seconds < 60 ? `${seconds}s` : `${Math.round(seconds / 60)}m`
}

const W = 240
const H = 48

// One series over the last few minutes. max fixes the scale, so a calm
// CPU doesn't look busy by stretching to fill the chart.
export function Sparkline({
  values,
  max,
  label,
  format,
  interval,
  pointLabel,
}: {
  values: (number | undefined)[]
  max: number
  label: string
  format: (v: number) => string
  // Seconds between samples.
  interval: number
  // Names a point in place of how long ago it was sampled.
  pointLabel?: (i: number) => string
}) {
  const id = useId()
  const [hover, setHover] = useState<number | null>(null)
  const n = values.length
  if (n < 2 || max <= 0) {
    return <div className="h-12 rounded-md bg-zinc-950/2.5 dark:bg-white/2.5" aria-hidden="true" />
  }
  const x = (i: number) => (i / (n - 1)) * W
  const y = (v: number) => H - 1 - (Math.min(Math.max(v, 0), max) / max) * (H - 2)
  // Gaps stay gaps instead of joining across missing samples.
  let line = ''
  let area = ''
  let start: number | null = null
  values.forEach((v, i) => {
    if (v === undefined) {
      if (start !== null) area += `L${x(i - 1)},${H}L${x(start)},${H}Z`
      start = null
      return
    }
    const cmd = start === null ? 'M' : 'L'
    line += `${cmd}${x(i)},${y(v)}`
    area += start === null ? `M${x(i)},${H}L${x(i)},${y(v)}` : `L${x(i)},${y(v)}`
    if (start === null) start = i
  })
  if (start !== null) area += `L${x(n - 1)},${H}Z`
  const hv = hover !== null ? values[hover] : undefined

  return (
    <div className="relative">
      <svg
        viewBox={`0 0 ${W} ${H}`}
        preserveAspectRatio="none"
        className="block h-12 w-full overflow-visible text-sky-500"
        onPointerMove={(e) => {
          const r = e.currentTarget.getBoundingClientRect()
          setHover(Math.round(((e.clientX - r.left) / r.width) * (n - 1)))
        }}
        onPointerLeave={() => setHover(null)}
      >
        <title>{pointLabel ? label : `${label} over the last ${ago((n - 1) * interval)}`}</title>
        <defs>
          <linearGradient id={id} x1="0" x2="0" y1="0" y2="1">
            <stop offset="0" stopColor="currentColor" stopOpacity="0.25" />
            <stop offset="1" stopColor="currentColor" stopOpacity="0" />
          </linearGradient>
        </defs>
        <line
          x1="0"
          x2={W}
          y1={H - 0.5}
          y2={H - 0.5}
          className="stroke-zinc-950/10 dark:stroke-white/10"
          vectorEffect="non-scaling-stroke"
        />
        <path d={area} fill={`url(#${id})`} />
        <path
          d={line}
          fill="none"
          stroke="currentColor"
          strokeWidth="2"
          strokeLinejoin="round"
          strokeLinecap="round"
          vectorEffect="non-scaling-stroke"
        />
        {hover !== null && (
          <line
            x1={x(hover)}
            x2={x(hover)}
            y1="0"
            y2={H}
            className="stroke-zinc-950/30 dark:stroke-white/30"
            vectorEffect="non-scaling-stroke"
          />
        )}
      </svg>
      {hover !== null && (
        <div
          className="pointer-events-none absolute -top-8 rounded-md bg-zinc-900 px-2 py-1 text-xs/5 whitespace-nowrap text-white tabular-nums shadow-sm dark:bg-zinc-700"
          style={{
            left: `${(hover / (n - 1)) * 100}%`,
            transform: `translateX(${hover / (n - 1) > 0.5 ? '-100%' : '0'})`,
          }}
        >
          {hv === undefined ? 'No data' : format(hv)}
          <span className="text-zinc-400">
            {' · '}
            {pointLabel ? pointLabel(hover) : `${ago((n - 1 - hover) * interval)} ago`}
          </span>
        </div>
      )}
    </div>
  )
}
