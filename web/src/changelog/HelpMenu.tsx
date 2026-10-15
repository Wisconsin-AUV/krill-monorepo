import { useQuery } from '@connectrpc/connect-query'
import { QuestionMarkCircleIcon } from '@heroicons/react/20/solid'
import { useState } from 'react'
import {
  Dropdown,
  DropdownButton,
  DropdownDescription,
  DropdownDivider,
  DropdownHeading,
  DropdownItem,
  DropdownLabel,
  DropdownMenu,
  DropdownSection,
} from '@/components/ui/Dropdown'
import { NavbarItem } from '@/components/ui/Navbar'
import { HealthService } from '@/gen/krill/v1/health_pb'
import { headline, releaseOf } from './parse'
import { useReleases } from './releases'
import { useChangelog } from './store'

function Dot({ current }: { current?: boolean }) {
  return (
    <svg data-slot="icon" viewBox="0 0 16 16" fill="currentColor" aria-hidden="true">
      <circle cx="8" cy="8" r={current ? 3.5 : 2.5} />
    </svg>
  )
}

export function HelpMenu() {
  const { data: health } = useQuery(HealthService.method.check, {})
  const running = health ? releaseOf(health.version) : null
  const { seen, show, markSeen } = useChangelog()
  // Loading waits for the first hover so the changelog stays out of the
  // first page load.
  const [wanted, setWanted] = useState(false)
  const { data: releases } = useReleases(wanted)
  const recent = (releases ?? []).filter((r) => r.version !== 'Unreleased').slice(0, 3)
  const unread = running !== null && seen !== running

  return (
    <Dropdown>
      <DropdownButton
        as={NavbarItem}
        aria-label={unread ? "What's new, new release" : "What's new"}
        onPointerEnter={() => setWanted(true)}
        onFocus={() => setWanted(true)}
        onClick={() => {
          setWanted(true)
          if (running) markSeen(running)
        }}
      >
        <QuestionMarkCircleIcon data-slot="icon" />
        {unread && (
          <span className="absolute top-1.5 right-1.5 size-2 rounded-full bg-sky-500 ring-2 ring-zinc-100 dark:ring-zinc-950" />
        )}
      </DropdownButton>
      <DropdownMenu anchor="bottom end" className="min-w-72">
        <DropdownSection>
          <DropdownHeading>What's new</DropdownHeading>
          {recent.map((r, i) => (
            <DropdownItem key={r.version} onClick={() => show(r.version)}>
              <Dot current={i === 0} />
              <DropdownLabel>{r.version}</DropdownLabel>
              <DropdownDescription className="max-w-60 truncate">{headline(r)}</DropdownDescription>
            </DropdownItem>
          ))}
          <DropdownItem onClick={() => show()}>
            <Dot />
            <DropdownLabel>Full changelog</DropdownLabel>
          </DropdownItem>
        </DropdownSection>
        {health && (
          <>
            <DropdownDivider />
            <div className="col-span-full px-3.5 py-1 text-xs/5 text-zinc-500 tabular-nums sm:px-3 dark:text-zinc-400">
              Krill {health.version}
            </div>
          </>
        )}
      </DropdownMenu>
    </Dropdown>
  )
}
