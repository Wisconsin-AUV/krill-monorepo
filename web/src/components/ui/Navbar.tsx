import * as Headless from '@headlessui/react'
import { AnimatePresence, LayoutGroup, motion } from 'motion/react'
import React, { forwardRef, useEffect, useId, useState } from 'react'
import { NavLink, useLocation } from 'react-router'
import { clsx } from 'clsx'
import { TouchTarget } from './Button'

export function Navbar({ className, ...props }: React.ComponentPropsWithoutRef<'nav'>) {
  return <nav {...props} className={clsx(className, 'flex flex-1 items-center gap-4 py-2.5')} />
}

export function NavbarDivider({ className, ...props }: React.ComponentPropsWithoutRef<'div'>) {
  return (
    <div
      aria-hidden="true"
      {...props}
      className={clsx(className, 'h-6 w-px bg-zinc-950/10 dark:bg-white/10')}
    />
  )
}

export function NavbarSection({ className, ...props }: React.ComponentPropsWithoutRef<'div'>) {
  const id = useId()

  return (
    <LayoutGroup id={id}>
      <div {...props} className={clsx(className, 'flex items-center gap-3')} />
    </LayoutGroup>
  )
}

export function NavbarSpacer({ className, ...props }: React.ComponentPropsWithoutRef<'div'>) {
  return <div aria-hidden="true" {...props} className={clsx(className, '-ml-4 flex-1')} />
}

export const NavbarItem = forwardRef(function NavbarItem(
  {
    className,
    children,
    includeSubPaths,
    accent,
    ...props
  }: {
    className?: string
    children: React.ReactNode
    includeSubPaths?: boolean
    accent?: boolean
  } & (
    | Omit<Headless.ButtonProps, 'as' | 'className'>
    | Omit<React.ComponentPropsWithoutRef<typeof NavLink>, 'className'>
  ),
  ref: React.ForwardedRef<HTMLAnchorElement | HTMLButtonElement>,
) {
  const classes = clsx(
    // Base
    'relative flex min-w-0 items-center gap-3 rounded-lg p-2 text-left text-base/6 font-medium text-zinc-950 sm:text-sm/5',
    // Leading icon/icon-only
    '*:data-[slot=icon]:size-6 *:data-[slot=icon]:shrink-0 *:data-[slot=icon]:fill-zinc-500 sm:*:data-[slot=icon]:size-5',
    // Trailing icon (down chevron or similar)
    '*:not-nth-2:last:data-[slot=icon]:ml-auto *:not-nth-2:last:data-[slot=icon]:size-5 sm:*:not-nth-2:last:data-[slot=icon]:size-4',
    // Avatar
    '*:data-[slot=avatar]:-m-0.5 *:data-[slot=avatar]:size-7 *:data-[slot=avatar]:[--avatar-radius:var(--radius)] *:data-[slot=avatar]:[--ring-opacity:10%] sm:*:data-[slot=avatar]:size-6',
    // Hover
    'data-hover:bg-zinc-950/5 data-hover:*:data-[slot=icon]:fill-zinc-950',
    // Active
    'data-active:bg-zinc-950/5 data-active:*:data-[slot=icon]:fill-zinc-950',
    // Dark mode
    'dark:text-white dark:*:data-[slot=icon]:fill-zinc-400',
    'dark:data-hover:bg-white/5 dark:data-hover:*:data-[slot=icon]:fill-white',
    'dark:data-active:bg-white/5 dark:data-active:*:data-[slot=icon]:fill-white',
  )
  const location = useLocation()

  let current: boolean

  const cleanLocation = location.pathname.replace(/\/$/, '')

  if ('to' in props && typeof props.to === 'string') {
    const cleanTo = props.to.replace(/\/$/, '')
    if (includeSubPaths) {
      current = cleanLocation.includes(cleanTo)
    } else {
      current = cleanLocation === cleanTo
    }
  } else {
    current = false
  }

  return (
    <span className={clsx(className, 'relative')}>
      <AnimatePresence>
        {current && (
          <motion.span
            layoutId="current-indicator"
            className={clsx([
              'absolute inset-x-2 -bottom-2.5 h-0.5',
              accent
                ? 'bg-sky-500/50 dark:bg-sky-500/25'
                : 'rounded-full bg-zinc-950 dark:bg-white',
            ])}
          />
        )}
      </AnimatePresence>

      {'to' in props ? (
        <Headless.DataInteractive>
          <NavLink
            {...props}
            className={classes}
            data-current={current ? 'true' : undefined}
            ref={ref as React.ForwardedRef<HTMLAnchorElement>}
          >
            <TouchTarget>{children}</TouchTarget>
          </NavLink>
        </Headless.DataInteractive>
      ) : (
        <Headless.Button
          {...props}
          className={clsx('cursor-default', classes)}
          data-current={current ? 'true' : undefined}
          ref={ref}
        >
          <TouchTarget>{children}</TouchTarget>
        </Headless.Button>
      )}
    </span>
  )
})

interface NavbarAlertProps {
  className?: string
  text: string
  icon: React.ReactNode
  autoOpen?: boolean
  flash?: boolean
}

export const NavbarAlert = forwardRef<HTMLButtonElement, NavbarAlertProps>(
  ({ className, text, icon, autoOpen = false, flash = false, ...props }, ref) => {
    const [isHovered, setIsHovered] = useState(false)
    const [isExpanded, setIsExpanded] = useState(autoOpen)

    useEffect(() => {
      if (autoOpen) {
        const timer = setTimeout(() => setIsExpanded(false), 5000)
        return () => clearTimeout(timer)
      }
      return undefined
    }, [autoOpen])

    return (
      <motion.div
        className={clsx(
          'relative flex items-center overflow-hidden rounded-lg p-2 text-base font-medium text-zinc-950 sm:text-sm',
          'dark:text-white dark:hover:bg-white/5',
          className,
        )}
        onHoverStart={() => setIsHovered(true)}
        onHoverEnd={() => setIsHovered(false)}
      >
        <Headless.Button {...props} className="flex cursor-default items-center gap-2" ref={ref}>
          <span className={clsx(['size-6 sm:size-4', flash && 'animate-pulse'])}>{icon}</span>
          <motion.span
            initial={{ width: 0, opacity: 0 }}
            animate={{
              width: isExpanded || isHovered ? 'auto' : 0,
              opacity: isExpanded || isHovered ? 1 : 0,
            }}
            transition={{ duration: 0.3, ease: 'easeInOut' }}
            className="overflow-hidden whitespace-nowrap"
          >
            {text}
          </motion.span>
        </Headless.Button>
      </motion.div>
    )
  },
)

export function NavbarLabel({ className, ...props }: React.ComponentPropsWithoutRef<'span'>) {
  return <span {...props} className={clsx(className, 'truncate')} />
}
