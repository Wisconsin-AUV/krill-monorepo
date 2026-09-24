import { clsx } from 'clsx'
import { useMemo } from 'react'
import type { ContributionDay } from '@/gen/krill/v1/stats_pb'
import { buildCalendar, level, type CalendarDay } from '@/lib/calendar'
import { plural } from '@/lib/format'

const levels = [
  'bg-zinc-950/5 dark:bg-white/5',
  'bg-sky-500/25',
  'bg-sky-500/50',
  'bg-sky-500/75',
  'bg-sky-500',
]

const monthFormat = new Intl.DateTimeFormat(undefined, { month: 'short' })
const dayFormat = new Intl.DateTimeFormat(undefined, {
  weekday: 'short',
  month: 'short',
  day: 'numeric',
  year: 'numeric',
})

function describe(day: CalendarDay) {
  return `${day.count === 0 ? 'No' : plural(day.count, 'contribution')} on ${dayFormat.format(day.date)}`
}

export function ContributionGraph({ days }: { days: ContributionDay[] }) {
  const weeks = useMemo(
    () => buildCalendar(new Map(days.map((d) => [d.date, Number(d.count)]))),
    [days],
  )
  const all = weeks.flat()
  const max = Math.max(0, ...all.map((d) => d.count))
  const total = all.reduce((n, d) => n + d.count, 0)

  return (
    <div>
      <div className="overflow-x-auto">
        <div className="inline-grid grid-flow-col grid-rows-[auto_repeat(7,--spacing(3))] gap-[3px] text-xs text-zinc-500 dark:text-zinc-400">
          <span />
          {['Mon', '', 'Wed', '', 'Fri', '', ''].map((label, i) => (
            <span key={i} className="pr-1 text-[10px]/3">
              {label}
            </span>
          ))}
          {weeks.map((week, w) => {
            const first = week[0].date
            return [
              <span
                key={`m${w}`}
                className="h-4 w-3 overflow-visible text-[10px]/4 whitespace-nowrap"
              >
                {first.getDate() <= 7 ? monthFormat.format(first) : ''}
              </span>,
              ...week.map((day) => (
                <span
                  key={day.key}
                  title={describe(day)}
                  className={clsx(
                    'size-3 rounded-[2px] hover:ring-1 hover:ring-zinc-950/50 dark:hover:ring-white/60',
                    levels[level(day.count, max)],
                  )}
                />
              )),
            ]
          })}
        </div>
      </div>
      <div className="mt-3 flex flex-wrap items-center justify-between gap-2 text-xs text-zinc-500 dark:text-zinc-400">
        <span className="tabular-nums">{plural(total, 'contribution')} in the last year</span>
        <span className="flex items-center gap-1">
          Less
          {levels.map((c) => (
            <span key={c} className={clsx('size-3 rounded-[2px]', c)} />
          ))}
          More
        </span>
      </div>
    </div>
  )
}
