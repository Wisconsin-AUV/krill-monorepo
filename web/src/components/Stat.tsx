import type React from 'react'

export function Stat({
  title,
  value,
  detail,
}: {
  title: string
  value: React.ReactNode
  detail?: React.ReactNode
}) {
  return (
    <div>
      <hr className="w-full border-t border-zinc-950/10 dark:border-white/10" />
      <div className="mt-6 text-lg/6 font-medium sm:text-sm/6">{title}</div>
      <div className="mt-3 text-3xl/8 font-semibold tabular-nums sm:text-2xl/8">{value}</div>
      {detail && (
        <div className="mt-3 text-sm/6 text-zinc-500 sm:text-xs/6 dark:text-zinc-400">{detail}</div>
      )}
    </div>
  )
}
