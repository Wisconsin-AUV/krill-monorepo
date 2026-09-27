import { clsx } from 'clsx'

export function Meter({
  value,
  className,
  label,
}: {
  value: number
  className?: string
  label: string
}) {
  return (
    <progress
      aria-label={label}
      max={1}
      value={Math.min(Math.max(value, 0), 1)}
      className={clsx(
        className,
        'block h-1.5 w-full appearance-none overflow-hidden rounded-full bg-zinc-950/5 dark:bg-white/10',
        '[&::-webkit-progress-bar]:bg-transparent',
        '[&::-webkit-progress-value]:rounded-full [&::-webkit-progress-value]:bg-sky-500 [&::-webkit-progress-value]:transition-[width] [&::-webkit-progress-value]:duration-500',
        '[&::-moz-progress-bar]:rounded-full [&::-moz-progress-bar]:bg-sky-500',
      )}
    />
  )
}
