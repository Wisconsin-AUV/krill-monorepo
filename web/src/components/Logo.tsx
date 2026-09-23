import { clsx } from 'clsx'

export function Logo({ className }: { className?: string }) {
  return (
    <span className={clsx(className, 'inline-flex items-center gap-2')}>
      <svg viewBox="0 0 24 24" className="size-6" aria-hidden="true">
        <path
          d="M4 14c0-4.4 3.6-8 8-8 3.3 0 6.2 2 7.4 5l1.6-.5-.7 2.3-2.2-.8.9-.5A6 6 0 0 0 6 14c0 1.7.7 3.2 1.8 4.3l-1.4 1.4A8 8 0 0 1 4 14Z"
          className="fill-sky-500"
        />
        <circle cx="14.5" cy="10.5" r="1.25" className="fill-zinc-950 dark:fill-white" />
        <path
          d="M9 17l-2 3M12 18l-1 3M15 17l1 3"
          className="stroke-sky-500"
          strokeWidth="1.5"
          strokeLinecap="round"
        />
      </svg>
      <span className="text-base font-semibold tracking-tight text-zinc-950 dark:text-white">
        krill
      </span>
    </span>
  )
}
