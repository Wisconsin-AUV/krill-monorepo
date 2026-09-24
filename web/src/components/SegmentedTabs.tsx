import { clsx } from 'clsx'
import { LayoutGroup, motion } from 'motion/react'
import { useId } from 'react'

export function SegmentedTabs({
  options,
  value,
  onChange,
}: {
  options: readonly { key: string; label: string }[]
  value: string
  onChange: (key: string) => void
}) {
  const id = useId()
  return (
    <LayoutGroup id={id}>
      <div className="inline-flex rounded-lg bg-zinc-950/5 p-0.5 dark:bg-white/5">
        {options.map((p) => (
          <button
            key={p.key}
            type="button"
            aria-pressed={p.key === value}
            onClick={() => onChange(p.key)}
            className={clsx(
              'relative rounded-md px-3 py-1 text-sm/6 font-medium transition-colors',
              p.key === value
                ? 'text-zinc-950 dark:text-white'
                : 'text-zinc-500 hover:text-zinc-950 dark:text-zinc-400 dark:hover:text-white',
            )}
          >
            {p.key === value && (
              <motion.span
                layoutId="segment-indicator"
                transition={{ type: 'spring', bounce: 0.15, duration: 0.3 }}
                className="absolute inset-0 rounded-md bg-white shadow-sm dark:bg-zinc-700"
              />
            )}
            <span className="relative">{p.label}</span>
          </button>
        ))}
      </div>
    </LayoutGroup>
  )
}
