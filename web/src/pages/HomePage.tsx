import { useQuery } from '@connectrpc/connect-query'
import { ArrowRightIcon, PlayIcon } from '@heroicons/react/20/solid'
import { CheckBadgeIcon, FilmIcon } from '@heroicons/react/24/outline'
import { Link } from 'react-router'
import { ClipStrip, ClipStripLegend } from '@/components/ClipStrip'
import { EmptyState } from '@/components/EmptyState'
import { Meter } from '@/components/Meter'
import { Stat } from '@/components/Stat'
import { Thumbnail } from '@/components/Thumbnail'
import { Avatar } from '@/components/ui/Avatar'
import { Button } from '@/components/ui/Button'
import { Heading, Subheading } from '@/components/ui/Heading'
import LoadingSpinner from '@/components/ui/LoadingSpinner'
import PageContentBlock from '@/components/ui/PageContentBlock'
import { Text } from '@/components/ui/Text'
import { QueueService, type GetQueueResponse, type OpenClip } from '@/gen/krill/v1/queue_pb'
import { Period, StatsService } from '@/gen/krill/v1/stats_pb'
import { initials, useUser } from '@/lib/auth'
import { errorMessage } from '@/lib/errors'
import { formatNumber, plural } from '@/lib/format'
import { takenByOther, useClaimNextClip } from '@/lib/queue'
import { ordinal, profilePath, timeZone } from '@/lib/stats'

function clipTitle(open: OpenClip) {
  return `${open.videoName} · Clip ${(open.clip?.index ?? 0) + 1}`
}

function NextUp({ queue }: { queue: GetQueueResponse }) {
  const claimNext = useClaimNextClip()
  const current = queue.openClips[0]

  if (current?.clip) {
    const { clip } = current
    return (
      <div className="flex flex-col gap-6 rounded-xl border border-zinc-950/10 p-5 sm:flex-row sm:items-center dark:border-white/10">
        <Thumbnail src={clip.thumbnailUrl} className="w-full shrink-0 sm:w-56" />
        <div className="min-w-0 flex-1">
          <div className="text-sm/6 text-zinc-500 dark:text-zinc-400">
            Pick up where you left off
          </div>
          <div className="mt-1 truncate text-lg/7 font-semibold">{clipTitle(current)}</div>
          <div className="mt-3 text-xs/5 text-zinc-500 tabular-nums dark:text-zinc-400">
            {formatNumber(clip.labeledFrameCount)} of {plural(clip.frameCount, 'frame')} done
          </div>
          <Meter
            value={clip.frameCount > 0 ? clip.labeledFrameCount / clip.frameCount : 0}
            label="Clip progress"
            className="mt-1 max-w-72"
          />
          <Button color="sky" to={`/clips/${clip.id}`} className="mt-5">
            <PlayIcon data-slot="icon" />
            Continue labeling
          </Button>
        </div>
      </div>
    )
  }

  if (queue.availableClips === 0) {
    return (
      <EmptyState
        icon={CheckBadgeIcon}
        title="Nothing left to label"
        description="Every clip is finished or being labeled."
      />
    )
  }

  return (
    <div className="flex flex-col items-start rounded-xl border border-zinc-950/10 p-6 dark:border-white/10">
      <div className="text-lg/7 font-semibold">Ready for a clip?</div>
      <Button
        color="sky"
        className="mt-5"
        disabled={claimNext.isPending}
        onClick={() => claimNext.mutate({})}
      >
        <PlayIcon data-slot="icon" />
        Start labeling
      </Button>
    </div>
  )
}

function WeekBoard() {
  const me = useUser()
  const { data } = useQuery(StatsService.method.getLeaderboard, {
    period: Period.WEEK,
    timeZone,
  })
  const entries = data?.entries.slice(0, 5) ?? []

  return (
    <div>
      <div className="flex items-baseline justify-between">
        <Subheading>This week</Subheading>
        <Link
          to="/leaderboard"
          className="text-sm/6 text-zinc-500 hover:text-zinc-950 dark:text-zinc-400 dark:hover:text-white"
        >
          Leaderboard
        </Link>
      </div>
      {entries.length === 0 ? (
        <Text className="mt-3">Nobody has labeled anything this week yet.</Text>
      ) : (
        <ol className="mt-3 divide-y divide-zinc-950/5 dark:divide-white/5">
          {entries.map((e) => (
            <li key={e.user?.id}>
              <Link
                to={profilePath(e.user?.username ?? '')}
                className="flex items-center gap-3 py-2 text-sm/6 hover:text-zinc-950 dark:hover:text-white"
              >
                <span className="w-4 text-right text-zinc-500 tabular-nums dark:text-zinc-400">
                  {e.rank}
                </span>
                <Avatar
                  initials={initials(e.user?.name ?? '')}
                  className="size-6 rounded-full bg-sky-600 [&>span]:text-[10px]"
                />
                <span className="min-w-0 flex-1 truncate font-medium">
                  {e.user?.id === me?.id ? 'You' : e.user?.name}
                </span>
                <span className="tabular-nums">{formatNumber(e.contributions?.total ?? 0)}</span>
              </Link>
            </li>
          ))}
        </ol>
      )}
    </div>
  )
}

function MyStats({ queue }: { queue: GetQueueResponse }) {
  const me = useUser()
  const { data } = useQuery(
    StatsService.method.getProfile,
    { username: me?.username ?? '', timeZone },
    { enabled: !!me },
  )
  const period = (p: Period) => data?.periods.find((x) => x.period === p)
  const today = period(Period.DAY)
  const week = period(Period.WEEK)
  const busy = queue.videos
    .flatMap((v) => v.clips)
    .filter((c) => c.labeledFrameCount < c.frameCount && takenByOther(c.claim, me?.id)).length

  return (
    <div className="mt-10 grid gap-8 sm:grid-cols-2 xl:grid-cols-4">
      <Stat
        title="Today"
        value={formatNumber(today?.contributions?.total ?? 0)}
        detail={today?.rank ? `${ordinal(today.rank)} on the team` : 'No labels yet today'}
      />
      <Stat
        title="This week"
        value={formatNumber(week?.contributions?.total ?? 0)}
        detail={
          week?.rank
            ? `${ordinal(week.rank)} of ${plural(week.contributors, 'labeler')}`
            : 'Unranked'
        }
      />
      <Stat
        title="Streak"
        value={plural(data?.currentStreak ?? 0, 'day')}
        detail={`Longest ${plural(data?.longestStreak ?? 0, 'day')}`}
      />
      <Stat
        title="Clips to label"
        value={formatNumber(queue.availableClips)}
        detail={
          busy > 0 ? `${plural(busy, 'clip')} being labeled by others` : 'Nobody else is labeling'
        }
      />
    </div>
  )
}

function OtherOpenClips({ clips }: { clips: OpenClip[] }) {
  if (clips.length === 0) return null
  return (
    <>
      <Subheading className="mt-14">Also in progress</Subheading>
      <ul className="mt-4 grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {clips.map((o) => (
          <li key={String(o.clip?.id)}>
            <Link
              to={`/clips/${o.clip?.id}`}
              className="flex items-center gap-4 rounded-lg p-2 hover:bg-zinc-950/[2.5%] dark:hover:bg-white/[2.5%]"
            >
              <Thumbnail src={o.clip?.thumbnailUrl} className="w-24 shrink-0" />
              <div className="min-w-0 flex-1">
                <div className="truncate text-sm/6 font-medium">{clipTitle(o)}</div>
                <div className="text-xs/5 text-zinc-500 tabular-nums dark:text-zinc-400">
                  {formatNumber(o.clip?.labeledFrameCount ?? 0)} of{' '}
                  {plural(o.clip?.frameCount ?? 0, 'frame')} done
                </div>
              </div>
              <ArrowRightIcon className="size-4 shrink-0 fill-zinc-400" />
            </Link>
          </li>
        ))}
      </ul>
    </>
  )
}

function Footage({ queue }: { queue: GetQueueResponse }) {
  return (
    <>
      <div className="mt-14 flex flex-wrap items-baseline justify-between gap-4">
        <Subheading>Footage</Subheading>
        <ClipStripLegend />
      </div>
      {queue.videos.length === 0 ? (
        <div className="mt-4">
          <EmptyState
            icon={FilmIcon}
            title="No footage yet"
            description="A developer needs to upload videos before you can label."
          />
        </div>
      ) : (
        <ul className="mt-4 space-y-6">
          {queue.videos.map((v) => {
            const frames = v.clips.reduce((n, c) => n + c.frameCount, 0)
            const labeled = v.clips.reduce((n, c) => n + c.labeledFrameCount, 0)
            return (
              <li key={String(v.id)}>
                <div className="mb-2 flex items-baseline justify-between gap-4 text-sm/6">
                  <Link
                    to={`/videos/${v.id}`}
                    className="truncate font-medium hover:text-sky-600 dark:hover:text-sky-400"
                  >
                    {v.name}
                  </Link>
                  <span className="shrink-0 text-xs text-zinc-500 tabular-nums dark:text-zinc-400">
                    {frames > 0 ? Math.round((labeled / frames) * 100) : 0}% ·{' '}
                    {plural(v.clips.length, 'clip')}
                  </span>
                </div>
                <ClipStrip clips={v.clips} />
              </li>
            )
          })}
        </ul>
      )}
    </>
  )
}

export function HomePage() {
  const me = useUser()
  const { data, isPending, error } = useQuery(
    QueueService.method.getQueue,
    {},
    { refetchInterval: 30_000 },
  )
  const firstName = me?.name.split(/\s+/)[0]

  return (
    <PageContentBlock title="Home · Krill">
      <Heading>{firstName ? `Welcome back, ${firstName}` : 'Welcome back'}</Heading>
      {isPending ? (
        <div className="flex justify-center py-16">
          <LoadingSpinner />
        </div>
      ) : error ? (
        <Text className="mt-4 text-red-600 dark:text-red-400">
          Could not load your queue: {errorMessage(error)}
        </Text>
      ) : (
        <>
          <div className="mt-8 grid gap-10 lg:grid-cols-3">
            <div className="lg:col-span-2">
              <NextUp queue={data} />
            </div>
            <WeekBoard />
          </div>
          <MyStats queue={data} />
          <OtherOpenClips clips={data.openClips.slice(1)} />
          <Footage queue={data} />
        </>
      )}
    </PageContentBlock>
  )
}
