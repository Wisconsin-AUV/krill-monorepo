import React from 'react'

import { clsx } from 'clsx'

export function DescriptionList({ className, ...props }: React.ComponentPropsWithoutRef<'dl'>) {
  return (
    <dl
      {...props}
      className={clsx(
        className,
        'grid grid-cols-1 text-base/6 sm:grid-cols-[min(50%,theme(spacing.80))_auto] sm:text-sm/6',
      )}
    />
  )
}

export function DescriptionTerm({ className, ...props }: React.ComponentPropsWithoutRef<'dt'>) {
  return (
    <dt
      {...props}
      className={clsx(
        className,
        'col-start-1 border-t border-zinc-950/5 pt-3 text-zinc-500 first:border-none sm:border-t sm:border-zinc-950/5 sm:py-3 dark:border-white/5 dark:text-zinc-400 sm:dark:border-white/5',
      )}
    />
  )
}

export function DescriptionDetails({ className, ...props }: React.ComponentPropsWithoutRef<'dd'>) {
  return (
    <dd
      {...props}
      className={clsx(
        className,
        'pt-1 pb-3 text-zinc-950 sm:border-t sm:border-zinc-950/5 sm:py-3 dark:text-white dark:sm:border-white/5 sm:[&:nth-child(2)]:border-none',
      )}
    />
  )
}

// Wrapper for a row in the description list
export const DescriptionItem: React.FC<{
  term: React.ReactNode
  details: React.ReactNode
  termClassName?: string
  detailsClassName?: string
  itemClassName?: string
}> = ({ term, details, termClassName, detailsClassName, itemClassName }) => {
  return (
    <div className={clsx(itemClassName, 'pt-3 first:pt-0')}>
      {' '}
      {/* Adjusted for block layout */}
      <DescriptionTerm className={termClassName}>{term}</DescriptionTerm>
      <DescriptionDetails className={detailsClassName}>{details}</DescriptionDetails>
    </div>
  )
}
