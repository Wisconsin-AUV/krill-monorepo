import { clsx } from 'clsx'
import type React from 'react'
import { Button } from './Button'

export function Pagination({
  'aria-label': ariaLabel = 'Page navigation',
  className,
  ...props
}: React.ComponentPropsWithoutRef<'nav'>) {
  return <nav aria-label={ariaLabel} {...props} className={clsx(className, 'flex gap-x-2')} />
}

export function PaginationPrevious({
  to = null,
  onClick,
  children = 'Previous',
}: {
  to?: string | null
  onClick?: (e: React.MouseEvent) => void
  children?: React.ReactNode
}) {
  return (
    <span className="grow basis-0">
      <Button
        {...(to === null ? { disabled: true } : { to })}
        {...(onClick ? { onClick } : {})}
        plain
        aria-label="Previous page"
      >
        <svg
          className="stroke-current"
          data-slot="icon"
          viewBox="0 0 16 16"
          fill="none"
          aria-hidden="true"
        >
          <path
            d="M2.75 8H13.25M2.75 8L5.25 5.5M2.75 8L5.25 10.5"
            strokeWidth={1.5}
            strokeLinecap="round"
            strokeLinejoin="round"
          />
        </svg>
        {children}
      </Button>
    </span>
  )
}

export function PaginationNext({
  to = null,
  onClick,
  children = 'Next',
}: {
  to?: string | null
  onClick?: (e: React.MouseEvent) => void
  children?: React.ReactNode
}) {
  return (
    <span className="flex grow basis-0 justify-end">
      <Button
        {...(to === null ? { disabled: true } : { to })}
        {...(onClick ? { onClick } : {})}
        plain
        aria-label="Next page"
      >
        {children}
        <svg
          className="stroke-current"
          data-slot="icon"
          viewBox="0 0 16 16"
          fill="none"
          aria-hidden="true"
        >
          <path
            d="M13.25 8L2.75 8M13.25 8L10.75 10.5M13.25 8L10.75 5.5"
            strokeWidth={1.5}
            strokeLinecap="round"
            strokeLinejoin="round"
          />
        </svg>
      </Button>
    </span>
  )
}

export function PaginationList({ children }: { children: React.ReactNode }) {
  return <span className="hidden items-baseline gap-x-2 sm:flex">{children}</span>
}

export function PaginationPage({
  to,
  onClick,
  children,
  current = false,
}: {
  to?: string
  onClick?: (e: React.MouseEvent) => void
  children: string
  current?: boolean
}) {
  return (
    <Button
      {...(to ? { to } : {})}
      {...(onClick ? { onClick } : {})}
      plain
      aria-label={`Page ${children}`}
      aria-current={current ? 'page' : undefined}
      className={clsx(
        'min-w-[2.25rem] before:absolute before:-inset-px before:rounded-lg',
        current && 'before:bg-zinc-950/5 dark:before:bg-white/10',
      )}
    >
      <span className="-mx-0.5">{children}</span>
    </Button>
  )
}

export function PaginationGap() {
  return (
    <div
      aria-hidden="true"
      className="w-[2.25rem] text-center text-sm/6 font-semibold text-zinc-950 select-none dark:text-white"
    >
      &hellip;
    </div>
  )
}
