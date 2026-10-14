import * as Headless from '@headlessui/react'
import { clsx } from 'clsx'

export function Switch({
  className,
  ...props
}: { className?: string } & Omit<Headless.SwitchProps, 'as' | 'className' | 'children'>) {
  return (
    <Headless.Switch
      data-slot="control"
      {...props}
      className={clsx(
        className,
        'group relative isolate inline-flex h-6 w-10 shrink-0 cursor-default rounded-full p-[3px] sm:h-5 sm:w-8',
        'transition duration-0 ease-in-out data-changing:duration-200',
        'forced-colors:outline forced-colors:[--switch-bg:Highlight] dark:forced-colors:[--switch-bg:Highlight]',
        'bg-zinc-200 ring-1 ring-black/5 ring-inset dark:bg-white/5 dark:ring-white/15',
        'data-checked:bg-sky-500 data-checked:ring-sky-600/90 dark:data-checked:bg-sky-500 dark:data-checked:ring-transparent',
        'focus:not-data-focus:outline-hidden data-focus:outline-2 data-focus:outline-offset-2 data-focus:outline-sky-500',
        'data-hover:ring-black/15 data-hover:data-checked:ring-sky-600 dark:data-hover:ring-white/25',
        'data-disabled:bg-zinc-200 data-disabled:opacity-50 data-disabled:data-checked:bg-zinc-200 data-disabled:data-checked:ring-black/5 dark:data-disabled:bg-white/15 dark:data-disabled:data-checked:bg-white/15',
      )}
    >
      <span
        aria-hidden="true"
        className={clsx(
          'pointer-events-none relative inline-block size-4.5 rounded-full sm:size-3.5',
          'translate-x-0 transition duration-200 ease-in-out',
          'border border-transparent bg-white shadow-sm ring-1 ring-black/5',
          'group-data-checked:translate-x-4 sm:group-data-checked:translate-x-3',
          'group-data-disabled:group-data-checked:bg-white group-data-disabled:group-data-checked:ring-black/5',
        )}
      />
    </Headless.Switch>
  )
}
