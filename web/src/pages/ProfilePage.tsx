import { useQuery } from '@connectrpc/connect-query'
import { timestampDate } from '@bufbuild/protobuf/wkt'
import { PencilIcon } from '@heroicons/react/16/solid'
import { useParams } from 'react-router'
import { ContributionGraph } from '@/components/ContributionGraph'
import { Stat } from '@/components/Stat'
import { Avatar } from '@/components/ui/Avatar'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { Heading, Subheading } from '@/components/ui/Heading'
import LoadingSpinner from '@/components/ui/LoadingSpinner'
import PageContentBlock from '@/components/ui/PageContentBlock'
import { Text } from '@/components/ui/Text'
import { StatsService, type GetProfileResponse } from '@/gen/krill/v1/stats_pb'
import { initials, useRoleLabel, useUser } from '@/lib/auth'
import { errorMessage } from '@/lib/errors'
import { formatNumber, plural } from '@/lib/format'
import { ordinal, periods, timeZone } from '@/lib/stats'

const joinedFormat = new Intl.DateTimeFormat(undefined, { month: 'long', year: 'numeric' })
const bestDayFormat = new Intl.DateTimeFormat(undefined, {
  month: 'short',
  day: 'numeric',
  year: 'numeric',
})

function PeriodStats({ data }: { data: GetProfileResponse }) {
  return (
    <div className="mt-10 grid gap-8 sm:grid-cols-2 xl:grid-cols-4">
      {periods.map((p) => {
        const s = data.periods.find((x) => x.period === p.period)
        return (
          <Stat
            key={p.key}
            title={p.label}
            value={formatNumber(s?.contributions?.total ?? 0)}
            detail={
              s?.rank ? `${ordinal(s.rank)} of ${plural(s.contributors, 'labeler')}` : 'Unranked'
            }
          />
        )
      })}
    </div>
  )
}

function Highlights({ data }: { data: GetProfileResponse }) {
  const all = data.periods.find((p) => p.period === periods[3].period)?.contributions
  // The API sends dates as YYYY-MM-DD, which Date would read as UTC midnight.
  const best = data.bestDay && new Date(`${data.bestDay.date}T00:00`)
  return (
    <div className="mt-10 grid gap-8 sm:grid-cols-2 xl:grid-cols-4">
      <Stat
        title="Current streak"
        value={plural(data.currentStreak, 'day')}
        detail={`Longest ${plural(data.longestStreak, 'day')}`}
      />
      <Stat
        title="Active days"
        value={formatNumber(data.activeDays)}
        detail={`Across ${plural(data.clips, 'clip')}`}
      />
      <Stat
        title="Best day"
        value={data.bestDay ? formatNumber(data.bestDay.count) : '–'}
        detail={best ? bestDayFormat.format(best) : 'No contributions yet'}
      />
      <Stat
        title="Boxes · frames"
        value={`${formatNumber(all?.boxes ?? 0)} · ${formatNumber(all?.frames ?? 0)}`}
        detail="Drawn · marked done"
      />
    </div>
  )
}

function LabelTypes({ data }: { data: GetProfileResponse }) {
  if (data.labelTypes.length === 0) return null
  const max = Number(data.labelTypes[0].boxes)
  return (
    <>
      <Subheading className="mt-14">Boxes by label</Subheading>
      <ul className="mt-4 space-y-3">
        {data.labelTypes.map((t) => (
          <li key={t.labelTypeId} className="grid grid-cols-[8rem_1fr_4rem] items-center gap-4">
            <span className="flex items-center gap-2 truncate text-sm/6">
              <span className="size-2.5 shrink-0 rounded-full" style={{ background: t.color }} />
              {t.name}
            </span>
            <span className="h-2 rounded-full bg-zinc-950/5 dark:bg-white/5">
              <span
                className="block h-full rounded-full"
                style={{ width: `${(Number(t.boxes) / max) * 100}%`, background: t.color }}
              />
            </span>
            <span className="text-right text-sm/6 tabular-nums">{formatNumber(t.boxes)}</span>
          </li>
        ))}
      </ul>
    </>
  )
}

export function ProfilePage() {
  const { username = '' } = useParams()
  const me = useUser()
  const roleLabel = useRoleLabel()
  const { data, isPending, error } = useQuery(StatsService.method.getProfile, {
    username,
    timeZone,
  })
  const user = data?.user

  return (
    <PageContentBlock title={`${user?.name ?? username} · Krill`}>
      {isPending ? (
        <div className="flex justify-center py-16">
          <LoadingSpinner />
        </div>
      ) : error ? (
        <Text className="text-red-600 dark:text-red-400">
          Could not load @{username}: {errorMessage(error)}
        </Text>
      ) : (
        <>
          <div className="flex flex-wrap items-center justify-between gap-4">
            <div className="flex items-center gap-5">
              <Avatar
                initials={initials(user?.name ?? '')}
                className="size-16 rounded-full bg-sky-600 [&>span]:text-xl"
              />
              <div>
                <div className="flex items-center gap-3">
                  <Heading>{user?.name}</Heading>
                  {user && <Badge>{roleLabel(user.role)}</Badge>}
                </div>
                <Text className="mt-1">
                  @{user?.username}
                  {user?.createdAt &&
                    ` · Joined ${joinedFormat.format(timestampDate(user.createdAt))}`}
                </Text>
              </div>
            </div>
            {user?.id === me?.id && (
              <Button outline to="/account">
                <PencilIcon data-slot="icon" />
                Edit account
              </Button>
            )}
          </div>

          <PeriodStats data={data} />

          <Subheading className="mt-14">Contributions</Subheading>
          <div className="mt-4">
            <ContributionGraph days={data.days} />
          </div>

          <Highlights data={data} />
          <LabelTypes data={data} />
        </>
      )}
    </PageContentBlock>
  )
}
