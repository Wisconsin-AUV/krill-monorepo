import React from 'react'
import { clsx } from 'clsx'

type HeadingProps = {
  level?: 1 | 2 | 3 | 4 | 5 | 6
  huge?: boolean
  invert?: boolean
} & React.ComponentPropsWithoutRef<'h1' | 'h2' | 'h3' | 'h4' | 'h5' | 'h6'>

export function Heading({ className, level = 1, huge, invert, ...props }: HeadingProps) {
  const Element: `h${typeof level}` = `h${level}`

  return (
    <Element
      {...props}
      className={clsx(
        'font-semibold',
        invert ? 'text-white dark:text-zinc-950' : 'text-zinc-950 dark:text-white',
        huge ? 'text-2xl sm:text-3xl' : 'text-2xl/8 sm:text-xl/8',
        className,
      )}
    />
  )
}

export function Subheading({ className, level = 2, huge, invert, ...props }: HeadingProps) {
  const Element: `h${typeof level}` = `h${level}`

  return (
    <Element
      {...props}
      className={clsx(
        'font-semibold',
        invert ? 'text-white dark:text-zinc-950' : 'text-zinc-950 dark:text-white',
        huge ? 'text-lg/7 sm:text-base/6' : 'text-base/7 sm:text-sm/6',
        className,
      )}
    />
  )
}
