import { useQuery } from '@connectrpc/connect-query'
import { MagnifyingGlassIcon } from '@heroicons/react/20/solid'
import { useDeferredValue, useState } from 'react'
import { SessionsTable } from '@/components/SessionsTable'
import { QueryError } from '@/components/ui/Callout'
import { Heading } from '@/components/ui/Heading'
import { Input, InputGroup } from '@/components/ui/Input'
import LoadingSpinner from '@/components/ui/LoadingSpinner'
import PageContentBlock from '@/components/ui/PageContentBlock'
import { Text } from '@/components/ui/Text'
import { SessionService } from '@/gen/krill/v1/session_pb'
import { plural } from '@/lib/format'
import { isActive, useRevokeSession } from '@/lib/sessions'

export function SessionsPage() {
  const { data, isPending, error } = useQuery(SessionService.method.listSessions, {})
  const revoke = useRevokeSession()
  const [search, setSearch] = useState('')
  const query = useDeferredValue(search.trim().toLowerCase())
  const all = data?.sessions ?? []
  const sessions = all.filter(
    (s) =>
      !query ||
      s.user?.name.toLowerCase().includes(query) ||
      s.user?.username.includes(query) ||
      s.ip.includes(query) ||
      s.device.toLowerCase().includes(query),
  )
  const active = all.filter((s) => isActive(s)).length
  const people = new Set(all.map((s) => s.user?.id)).size

  return (
    <PageContentBlock title="Sessions · Admin · Krill">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <Heading>Sessions</Heading>
          {data && (
            <Text className="mt-1">
              {plural(all.length, 'session')} · {plural(people, 'person', 'people')} · {active}{' '}
              active now
            </Text>
          )}
        </div>
        <div className="w-full sm:max-w-xs">
          <InputGroup>
            <MagnifyingGlassIcon data-slot="icon" />
            <Input
              type="search"
              aria-label="Search sessions"
              placeholder="Search"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
            />
          </InputGroup>
        </div>
      </div>

      {isPending ? (
        <div className="flex justify-center py-16">
          <LoadingSpinner />
        </div>
      ) : error ? (
        <QueryError title="Could not load sessions" error={error} className="mt-8" />
      ) : sessions.length === 0 ? (
        <p className="py-10 text-center text-sm/6 text-zinc-500 dark:text-zinc-400">
          {query ? `No sessions match “${search.trim()}”` : 'Nobody is signed in'}
        </p>
      ) : (
        <SessionsTable
          sessions={sessions}
          showUser
          onRevoke={(s) => revoke.mutate({ id: s.id })}
          revoking={revoke.isPending ? revoke.variables?.id : undefined}
          className="mt-8 [--gutter:--spacing(6)] lg:[--gutter:--spacing(10)]"
        />
      )}
    </PageContentBlock>
  )
}
