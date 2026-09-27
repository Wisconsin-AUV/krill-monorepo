import { useQuery } from '@connectrpc/connect-query'
import { timestampDate } from '@bufbuild/protobuf/wkt'
import { TrophyIcon } from '@heroicons/react/24/outline'
import { clsx } from 'clsx'
import { Link, useSearchParams } from 'react-router'
import { EmptyState } from '@/components/EmptyState'
import { SegmentedTabs } from '@/components/SegmentedTabs'
import { Stat } from '@/components/Stat'
import { Avatar } from '@/components/ui/Avatar'
import { Badge } from '@/components/ui/Badge'
import { Heading } from '@/components/ui/Heading'
import LoadingSpinner from '@/components/ui/LoadingSpinner'
import PageContentBlock from '@/components/ui/PageContentBlock'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/Table'
import { Text } from '@/components/ui/Text'
import { StatsService, type LeaderboardEntry } from '@/gen/krill/v1/stats_pb'
import { initials, useUser } from '@/lib/auth'
import { errorMessage } from '@/lib/errors'
import { formatNumber, plural } from '@/lib/format'
import { ordinal, periodFromKey, periods, profilePath, timeZone } from '@/lib/stats'

const medals = [
  'bg-amber-400 text-amber-950',
  'bg-zinc-300 text-zinc-800',
  'bg-orange-400 text-orange-950',
]

function RankBadge({ rank }: { rank: number }) {
  return (
    <span
      className={clsx(
        'inline-flex size-7 items-center justify-center rounded-full text-xs font-semibold tabular-nums',
        medals[rank - 1] ?? 'text-zinc-500 dark:text-zinc-400',
      )}
    >
      {rank}
    </span>
  )
}

function Podium({ entries }: { entries: LeaderboardEntry[] }) {
  return (
    <div className="mt-8 grid gap-4 sm:grid-cols-3">
      {entries.slice(0, 3).map((e) => (
        <Link
          key={e.user?.id}
          to={profilePath(e.user?.username ?? '')}
          className="flex items-center gap-4 rounded-xl border border-zinc-950/10 p-4 hover:bg-zinc-950/[2.5%] dark:border-white/10 dark:hover:bg-white/[2.5%]"
        >
          <Avatar
            initials={initials(e.user?.name ?? '')}
            className="size-12 shrink-0 rounded-full bg-sky-600"
          />
          <div className="min-w-0">
            <div className="flex items-center gap-2">
              <RankBadge rank={e.rank} />
              <span className="truncate font-medium text-zinc-950 dark:text-white">
                {e.user?.name}
              </span>
            </div>
            <div className="mt-1 text-2xl/8 font-semibold tabular-nums">
              {formatNumber(e.contributions?.total ?? 0)}
            </div>
          </div>
        </Link>
      ))}
    </div>
  )
}

export function LeaderboardPage() {
  const me = useUser()
  const [params, setParams] = useSearchParams()
  const period = periodFromKey(params.get('period'))
  const { data, isPending, error } = useQuery(StatsService.method.getLeaderboard, {
    period: period.period,
    timeZone,
  })
  const entries = data?.entries ?? []
  const mine = entries.find((e) => e.user?.id === me?.id)

  return (
    <PageContentBlock title="Leaderboard · Krill">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <Heading>Leaderboard</Heading>
          <Text className="mt-1">
            One point per box drawn and per frame marked done. Deleted boxes and reopened frames
            don't count.
          </Text>
        </div>
        <SegmentedTabs
          options={periods}
          value={period.key}
          onChange={(key) => setParams({ period: key }, { replace: true })}
        />
      </div>

      {isPending ? (
        <div className="flex justify-center py-16">
          <LoadingSpinner />
        </div>
      ) : error ? (
        <Text className="mt-8 text-red-600 dark:text-red-400">
          Could not load the leaderboard: {errorMessage(error)}
        </Text>
      ) : (
        <>
          <div className="mt-8 grid gap-8 sm:grid-cols-2 xl:grid-cols-4">
            <Stat
              title="Team total"
              value={formatNumber(data.team?.total ?? 0)}
              detail={data.since ? `Since ${timestampDate(data.since).toLocaleDateString()}` : ''}
            />
            <Stat title="Boxes" value={formatNumber(data.team?.boxes ?? 0)} />
            <Stat title="Frames done" value={formatNumber(data.team?.frames ?? 0)} />
            <Stat
              title="Your rank"
              value={mine ? ordinal(mine.rank) : '–'}
              detail={
                mine
                  ? `of ${plural(entries.length, 'labeler')}`
                  : `${plural(entries.length, 'labeler')} so far`
              }
            />
          </div>

          {entries.length === 0 ? (
            <div className="mt-10">
              <EmptyState
                icon={TrophyIcon}
                title="Nobody's on the board yet"
                description="Draw a box or finish a frame to take first place."
              />
            </div>
          ) : (
            <>
              <Podium entries={entries} />
              <Table className="mt-8 [--gutter:--spacing(6)] lg:[--gutter:--spacing(10)]">
                <TableHead>
                  <TableRow>
                    <TableHeader className="w-0">Rank</TableHeader>
                    <TableHeader>Labeler</TableHeader>
                    <TableHeader className="text-right">Boxes</TableHeader>
                    <TableHeader className="text-right">Frames</TableHeader>
                    <TableHeader className="text-right">Total</TableHeader>
                  </TableRow>
                </TableHead>
                <TableBody>
                  {entries.map((e) => (
                    <TableRow
                      key={e.user?.id}
                      href={profilePath(e.user?.username ?? '')}
                      title={`${e.user?.name}'s profile`}
                      className={clsx(e.user?.id === me?.id && 'bg-sky-500/5')}
                    >
                      <TableCell>
                        <RankBadge rank={e.rank} />
                      </TableCell>
                      <TableCell>
                        <div className="flex items-center gap-3">
                          <Avatar
                            initials={initials(e.user?.name ?? '')}
                            className="size-8 rounded-full bg-sky-600"
                          />
                          <div>
                            <div className="flex items-center gap-2 font-medium text-zinc-950 dark:text-white">
                              {e.user?.name}
                              {e.user?.id === me?.id && <Badge color="sky">You</Badge>}
                            </div>
                            <div className="text-zinc-500 dark:text-zinc-400">
                              @{e.user?.username}
                            </div>
                          </div>
                        </div>
                      </TableCell>
                      <TableCell className="text-right tabular-nums">
                        {formatNumber(e.contributions?.boxes ?? 0)}
                      </TableCell>
                      <TableCell className="text-right tabular-nums">
                        {formatNumber(e.contributions?.frames ?? 0)}
                      </TableCell>
                      <TableCell className="text-right font-semibold tabular-nums">
                        {formatNumber(e.contributions?.total ?? 0)}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </>
          )}
        </>
      )}
    </PageContentBlock>
  )
}
