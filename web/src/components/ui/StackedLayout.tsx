import { clsx } from 'clsx'
import React, { useState } from 'react'
import { MobileSidebar, OpenMenuIcon } from './MobileSidebar'
import { NavbarItem } from './Navbar'

export function StackedLayout({
  navbar,
  sidebar,
  fill = false,
  children,
}: React.PropsWithChildren<{
  navbar: React.ReactNode
  sidebar: React.ReactNode
  // Pins the layout to the viewport so the content can size itself to the
  // remaining height instead of scrolling the page.
  fill?: boolean
}>) {
  const [showSidebar, setShowSidebar] = useState(false)

  return (
    <div
      className={clsx(
        'relative isolate flex w-full flex-col bg-white lg:bg-zinc-100 dark:bg-zinc-900 dark:lg:bg-zinc-950',
        fill ? 'h-svh' : 'min-h-svh',
      )}
    >
      {/* Sidebar on mobile */}
      <MobileSidebar open={showSidebar} close={() => setShowSidebar(false)}>
        {sidebar}
      </MobileSidebar>

      {/* Navbar */}
      <header className="flex items-center px-4">
        <div className="py-2.5 lg:hidden">
          <NavbarItem onClick={() => setShowSidebar(true)} aria-label="Open navigation">
            <OpenMenuIcon />
          </NavbarItem>
        </div>
        <div className="min-w-0 flex-1">{navbar}</div>
      </header>

      {/* Content */}
      <main className={clsx('flex flex-1 flex-col pb-2 lg:px-2', fill && 'min-h-0')}>
        <div
          className={clsx(
            'grow bg-white lg:rounded-lg lg:shadow-xs lg:ring-1 lg:ring-zinc-950/5 dark:lg:bg-zinc-900 dark:lg:ring-white/10',
            fill && 'flex min-h-0 flex-col overflow-hidden',
          )}
        >
          <div className={clsx(fill && 'flex min-h-0 flex-1 flex-col')}>{children}</div>
        </div>
      </main>
    </div>
  )
}
