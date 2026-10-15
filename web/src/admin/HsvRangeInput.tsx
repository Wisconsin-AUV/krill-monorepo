import { create } from '@bufbuild/protobuf'
import { HsvRangeSchema, type HsvRange } from '@/gen/krill/v1/pipeline_pb'

// OpenCV's HSV: hue 0 to 179 (half degrees), saturation and value 0 to 255.
const H_MAX = 179
const SV_MAX = 255

function hsvCss(h: number, s: number, v: number) {
  const sat = s / SV_MAX
  const val = v / SV_MAX
  const l = val * (1 - sat / 2)
  const sl = l === 0 || l === 1 ? 0 : (val - l) / Math.min(l, 1 - l)
  return `hsl(${h * 2} ${Math.round(sl * 100)}% ${Math.round(l * 100)}%)`
}

// The hue in the middle of the range, going around when it wraps.
function midHue(r: HsvRange) {
  const span = r.hMin <= r.hMax ? r.hMax - r.hMin : r.hMax + H_MAX + 1 - r.hMin
  return (r.hMin + span / 2) % (H_MAX + 1)
}

const hueGradient = `linear-gradient(to right, ${Array.from({ length: 13 }, (_, i) => `hsl(${i * 30} 100% 50%)`).join(', ')})`

function DualSlider({
  label,
  min,
  max,
  lo,
  hi,
  wraps,
  gradient,
  onChange,
}: {
  label: string
  min: number
  max: number
  lo: number
  hi: number
  wraps: boolean
  gradient: string
  onChange: (lo: number, hi: number) => void
}) {
  const pct = (v: number) => ((v - min) / (max - min)) * 100
  // Shade what the range leaves out. A wrapped hue range leaves out the middle.
  const shades: [number, number][] =
    wraps && lo > hi
      ? [[pct(hi), pct(lo)]]
      : [
          [0, pct(lo)],
          [pct(hi), 100],
        ]
  const thumb =
    'pointer-events-none absolute inset-0 h-full w-full appearance-none bg-transparent outline-none ' +
    '[&::-webkit-slider-thumb]:pointer-events-auto [&::-webkit-slider-thumb]:size-4 [&::-webkit-slider-thumb]:appearance-none [&::-webkit-slider-thumb]:rounded-full [&::-webkit-slider-thumb]:bg-white [&::-webkit-slider-thumb]:shadow [&::-webkit-slider-thumb]:ring-1 [&::-webkit-slider-thumb]:ring-zinc-950/30 ' +
    '[&::-moz-range-thumb]:pointer-events-auto [&::-moz-range-thumb]:size-4 [&::-moz-range-thumb]:rounded-full [&::-moz-range-thumb]:border-0 [&::-moz-range-thumb]:bg-white [&::-moz-range-thumb]:shadow [&::-moz-range-thumb]:ring-1 [&::-moz-range-thumb]:ring-zinc-950/30 ' +
    'focus-visible:[&::-webkit-slider-thumb]:ring-2 focus-visible:[&::-webkit-slider-thumb]:ring-sky-500'
  const clamp = (v: number) => Math.min(Math.max(Math.round(v), min), max)
  const number =
    'w-14 rounded-md border border-zinc-950/10 bg-transparent px-1.5 py-0.5 text-right text-sm/6 tabular-nums dark:border-white/10'

  return (
    <div className="grid grid-cols-[1.5rem_3.5rem_1fr_3.5rem] items-center gap-3">
      <span className="text-sm/6 font-medium text-zinc-500 dark:text-zinc-400">{label}</span>
      <input
        type="number"
        aria-label={`${label} minimum`}
        className={number}
        min={min}
        max={max}
        value={lo}
        onChange={(e) => {
          const v = clamp(e.target.valueAsNumber || 0)
          onChange(v, wraps ? hi : Math.max(v, hi))
        }}
      />
      <div className="relative h-6">
        <div
          className="absolute inset-x-0 top-1/2 h-2.5 -translate-y-1/2 overflow-hidden rounded-full ring-1 ring-zinc-950/10 dark:ring-white/10"
          style={{ background: gradient }}
        >
          {shades.map(([a, b]) => (
            <div
              key={`${a}-${b}`}
              className="absolute inset-y-0 bg-zinc-950/70"
              style={{ left: `${a}%`, width: `${Math.max(b - a, 0)}%` }}
            />
          ))}
        </div>
        <input
          type="range"
          aria-label={`${label} minimum`}
          min={min}
          max={max}
          value={lo}
          className={thumb}
          onChange={(e) => {
            const v = Number(e.target.value)
            onChange(v, wraps ? hi : Math.max(v, hi))
          }}
        />
        <input
          type="range"
          aria-label={`${label} maximum`}
          min={min}
          max={max}
          value={hi}
          className={thumb}
          onChange={(e) => {
            const v = Number(e.target.value)
            onChange(wraps ? lo : Math.min(lo, v), v)
          }}
        />
      </div>
      <input
        type="number"
        aria-label={`${label} maximum`}
        className={number}
        min={min}
        max={max}
        value={hi}
        onChange={(e) => {
          const v = clamp(e.target.valueAsNumber || 0)
          onChange(wraps ? lo : Math.min(lo, v), v)
        }}
      />
    </div>
  )
}

export function HsvRangeInput({
  value: r,
  onChange,
}: {
  value: HsvRange
  onChange: (r: HsvRange) => void
}) {
  const set = (patch: Partial<HsvRange>) => onChange(create(HsvRangeSchema, { ...r, ...patch }))
  const h = midHue(r)
  const sMid = (r.sMin + r.sMax) / 2
  const vMid = (r.vMin + r.vMax) / 2
  return (
    <div className="space-y-2">
      <div className="flex items-center gap-2">
        {[
          [r.sMin, r.vMin],
          [sMid, vMid],
          [r.sMax, r.vMax],
        ].map(([s, v], i) => (
          <span
            key={i}
            className="size-6 rounded-md ring-1 ring-zinc-950/10 dark:ring-white/10"
            style={{ backgroundColor: hsvCss(h, s, v) }}
          />
        ))}
      </div>
      <DualSlider
        label="H"
        min={0}
        max={H_MAX}
        lo={r.hMin}
        hi={r.hMax}
        wraps
        gradient={hueGradient}
        onChange={(hMin, hMax) => set({ hMin, hMax })}
      />
      <DualSlider
        label="S"
        min={0}
        max={SV_MAX}
        lo={r.sMin}
        hi={r.sMax}
        wraps={false}
        gradient={`linear-gradient(to right, ${hsvCss(h, 0, vMid)}, ${hsvCss(h, SV_MAX, vMid)})`}
        onChange={(sMin, sMax) => set({ sMin, sMax })}
      />
      <DualSlider
        label="V"
        min={0}
        max={SV_MAX}
        lo={r.vMin}
        hi={r.vMax}
        wraps={false}
        gradient={`linear-gradient(to right, ${hsvCss(h, sMid, 0)}, ${hsvCss(h, sMid, SV_MAX)})`}
        onChange={(vMin, vMax) => set({ vMin, vMax })}
      />
    </div>
  )
}
