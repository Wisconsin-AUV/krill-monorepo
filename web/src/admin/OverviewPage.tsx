import { timestampDate } from '@bufbuild/protobuf/wkt'
import { useMutation, useQuery } from '@connectrpc/connect-query'
import { ArrowPathIcon } from '@heroicons/react/16/solid'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { QueryError } from '@/components/ui/Callout'
import { Heading, Subheading } from '@/components/ui/Heading'
import LoadingSpinner from '@/components/ui/LoadingSpinner'
import PageContentBlock from '@/components/ui/PageContentBlock'
import { Text, TextLink } from '@/components/ui/Text'
import {
  AdminService,
  type ServiceCheck,
  UpdateState,
  type VersionStatus,
} from '@/gen/krill/v1/admin_pb'
import { flash } from '@/lib/flash'
import { formatRelative, plural } from '@/lib/format'
import { REPO_URL } from '@/lib/links'
import { invalidateService } from '@/lib/queryClient'
import { ReleaseNotes } from '@/changelog/ReleaseNotes'
import { StatusIcon } from './StatusIcon'

function UpdateBadge({ version }: { version: VersionStatus }) {
  switch (version.state) {
    case UpdateState.UP_TO_DATE:
      return <Badge color="green">Up to date</Badge>
    case UpdateState.AVAILABLE:
      return <Badge color="sky">{version.latest} available</Badge>
    case UpdateState.AHEAD:
      return version.commitsAhead > 0 ? (
        <Badge color="indigo">{plural(version.commitsAhead, 'commit')} ahead</Badge>
      ) : null
    default:
      return null
  }
}

function Commit({ version }: { version: VersionStatus }) {
  if (!version.commit) return null
  return (
    <a
      href={`${REPO_URL}commit/${version.commit}`}
      target="_blank"
      rel="noreferrer"
      className="rounded-md bg-zinc-950/5 px-1.5 py-0.5 font-mono text-sm/5 text-zinc-700 hover:bg-zinc-950/10 sm:text-xs/5 dark:bg-white/5 dark:text-zinc-300 dark:hover:bg-white/10"
    >
      {version.commit}
    </a>
  )
}

function VersionPanel({ version, startedAt }: { version: VersionStatus; startedAt?: Date }) {
  const check = useMutation(AdminService.method.getStatus, {
    onSuccess: () => invalidateService(AdminService),
    onError: (err) => flash.error('Could not check for updates', err),
  })

  return (
    <section className="mt-8 rounded-xl ring-1 ring-zinc-950/10 dark:ring-white/10">
      <div className="flex flex-wrap items-start justify-between gap-6 p-6">
        <div>
          <p className="text-sm/6 text-zinc-500 dark:text-zinc-400">Running</p>
          <div className="mt-1 flex items-center gap-3">
            <span className="text-3xl/8 font-semibold text-zinc-950 tabular-nums dark:text-white">
              {version.release || (version.commit ? 'Development' : version.current)}
            </span>
            <Commit version={version} />
            <UpdateBadge version={version} />
            {version.dirty && <Badge color="amber">Uncommitted changes</Badge>}
          </div>
          <Text className="mt-2">
            {startedAt && <>Started {formatRelative(startedAt)}</>}
            {startedAt && version.checkedAt && ' · '}
            {version.checkedAt && <>Checked {formatRelative(timestampDate(version.checkedAt))}</>}
          </Text>
          {!version.checkEnabled && (
            <Text className="mt-2">
              Update checks are off in <TextLink to="/admin/settings">settings</TextLink>.
            </Text>
          )}
          {version.error && <Text className="mt-2">{capitalize(version.error)}.</Text>}
        </div>
        <div className="flex gap-3">
          {version.checkEnabled && (
            <Button
              outline
              onClick={() => check.mutate({ refresh: true })}
              disabled={check.isPending}
            >
              <ArrowPathIcon data-slot="icon" className={check.isPending ? 'animate-spin' : ''} />
              Check now
            </Button>
          )}
        </div>
      </div>
      {version.releases.length > 0 && (
        <div className="space-y-8 border-t border-zinc-950/5 p-6 dark:border-white/5">
          {version.releases.map((r, i) => (
            <ReleaseNotes key={r.version} release={r} expanded={i === 0} />
          ))}
        </div>
      )}
    </section>
  )
}

function capitalize(s: string) {
  return s.charAt(0).toUpperCase() + s.slice(1)
}

function Services({ checks }: { checks: ServiceCheck[] }) {
  return (
    <ul className="mt-4 divide-y divide-zinc-950/5 border-y border-zinc-950/5 dark:divide-white/5 dark:border-white/5">
      {checks.map((c) => (
        <li key={c.name} className="flex items-center gap-4 py-4">
          <StatusIcon status={c.status} className="shrink-0" />
          <div className="min-w-0 flex-1 sm:flex sm:items-baseline sm:gap-4">
            <p className="w-36 shrink-0 text-sm/6 font-medium text-zinc-950 dark:text-white">
              {c.name}
            </p>
            <p className="truncate text-sm/6 text-zinc-500 dark:text-zinc-400">{c.detail}</p>
          </div>
        </li>
      ))}
    </ul>
  )
}

export function OverviewPage() {
  const { data, isPending, error } = useQuery(AdminService.method.getStatus, {})

  return (
    <PageContentBlock title="Admin · Krill">
      <Heading>Overview</Heading>
      {isPending ? (
        <div className="flex justify-center py-16">
          <LoadingSpinner />
        </div>
      ) : error ? (
        <QueryError title="Could not load status" error={error} className="mt-8" />
      ) : (
        <>
          {data.version && (
            <VersionPanel
              version={data.version}
              startedAt={data.startedAt && timestampDate(data.startedAt)}
            />
          )}
          <Subheading className="mt-12">Services</Subheading>
          <Services checks={data.checks} />
        </>
      )}
    </PageContentBlock>
  )
}
