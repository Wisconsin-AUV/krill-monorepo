import { clsx } from 'clsx'
import { Link } from 'react-router'
import type { ClipProgress } from '@/gen/krill/v1/queue_pb'
import { useUser } from '@/lib/auth'
import { formatNumber } from '@/lib/format'
import { clipState, type ClipState } from '@/lib/queue'

const cellColors: Record<ClipState | 'claimed', string> = {
  todo: 'bg-zinc-950/10 dark:bg-white/10',
  progress: 'bg-amber-400/70',
  claimed: 'bg-sky-500',
  done: 'bg-emerald-500',
}

const legend = [
  { label: 'Not started', className: cellColors.todo },
  { label: 'In progress', className: cellColors.progress },
  { label: 'Being labeled', className: cellColors.claimed },
  { label: 'Done', className: cellColors.done },
]

export function ClipStripLegend() {
  return (
    <div className="flex flex-wrap gap-x-4 gap-y-1 text-xs text-zinc-500 dark:text-zinc-400">
      {legend.map((l) => (
        <span key={l.label} className="flex items-center gap-1.5">
          <span className={clsx('size-2.5 rounded-[2px]', l.className)} />
          {l.label}
        </span>
      ))}
    </div>
  )
}

function describe(clip: ClipProgress, me: string | undefined) {
  const parts = [
    `Clip ${clip.index + 1}`,
    `${formatNumber(clip.labeledFrameCount)} of ${formatNumber(clip.frameCount)} frames done`,
  ]
  if (clip.claim?.active) {
    parts.push(
      clip.claim.user?.id === me ? 'You are labeling' : `${clip.claim.user?.name} is labeling`,
    )
  }
  return parts.join(' · ')
}

export function ClipStrip({ clips }: { clips: ClipProgress[] }) {
  const me = useUser()?.id
  return (
    <div className="flex flex-wrap gap-1">
      {clips.map((c) => {
        const state = clipState(c.labeledFrameCount, c.frameCount)
        const label = describe(c, me)
        return (
          <Link
            key={String(c.id)}
            to={`/clips/${c.id}`}
            title={label}
            aria-label={label}
            className={clsx(
              'size-3.5 rounded-[3px] hover:ring-2 hover:ring-zinc-950/50 focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-sky-500 dark:hover:ring-white/60',
              cellColors[state !== 'done' && c.claim?.active ? 'claimed' : state],
            )}
          />
        )
      })}
    </div>
  )
}
