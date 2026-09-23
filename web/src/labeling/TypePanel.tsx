import { Cog6ToothIcon } from '@heroicons/react/16/solid'
import { clsx } from 'clsx'
import { Link } from 'react-router'
import type { LabelType } from '@/gen/krill/v1/label_pb'
import { useLabelStore } from './useLabelStore'

export function TypePanel({ types, counts }: { types: LabelType[]; counts: Map<bigint, number> }) {
  const activeTypeId = useLabelStore((s) => s.activeTypeId)
  const setActiveType = useLabelStore((s) => s.setActiveType)

  return (
    <aside className="flex w-56 shrink-0 flex-col border-r border-zinc-950/10 bg-white dark:border-white/10 dark:bg-zinc-900">
      <div className="flex items-center justify-between px-3 pt-3 pb-2">
        <h2 className="text-xs/6 font-medium text-zinc-500 dark:text-zinc-400">Label types</h2>
        <Link
          to="/labels"
          className="rounded p-1 text-zinc-500 hover:bg-zinc-950/5 hover:text-zinc-700 dark:hover:bg-white/5 dark:hover:text-zinc-300"
          title="Manage label types"
        >
          <Cog6ToothIcon className="size-4" />
        </Link>
      </div>
      <ul className="flex-1 space-y-0.5 overflow-y-auto px-2 pb-3">
        {types.map((t, i) => {
          const active = t.id === activeTypeId
          return (
            <li key={String(t.id)}>
              <button
                type="button"
                onClick={() => setActiveType(active ? null : t.id)}
                className={clsx(
                  'flex w-full items-center gap-2.5 rounded-md px-2 py-1.5 text-left text-sm/5 transition-colors',
                  active
                    ? 'bg-zinc-950/5 text-zinc-950 dark:bg-white/10 dark:text-white'
                    : 'text-zinc-700 hover:bg-zinc-950/5 dark:text-zinc-300 dark:hover:bg-white/5',
                )}
              >
                <span
                  className={clsx(
                    'size-2.5 shrink-0 rounded-full',
                    active &&
                      'ring-2 ring-zinc-950/40 ring-offset-1 ring-offset-white dark:ring-white/60 dark:ring-offset-zinc-900',
                  )}
                  style={{ backgroundColor: t.color }}
                />
                <span className="min-w-0 flex-1 truncate font-mono text-[13px]">{t.name}</span>
                {(counts.get(t.id) ?? 0) > 0 && (
                  <span className="text-xs text-zinc-500 tabular-nums">{counts.get(t.id)}</span>
                )}
                {i < 9 && (
                  <kbd className="w-5 rounded border border-zinc-950/10 text-center font-sans text-[11px] text-zinc-500 dark:border-white/10">
                    {i + 1}
                  </kbd>
                )}
              </button>
            </li>
          )
        })}
        {types.length === 0 && (
          <li className="px-2 py-4 text-sm/6 text-zinc-500 dark:text-zinc-400">
            No label types yet.{' '}
            <Link
              to="/labels"
              className="text-sky-600 hover:text-sky-500 dark:text-sky-400 dark:hover:text-sky-300"
            >
              Create some
            </Link>{' '}
            to start drawing.
          </li>
        )}
      </ul>
    </aside>
  )
}
