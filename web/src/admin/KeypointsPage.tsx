import { timestampDate } from '@bufbuild/protobuf/wkt'
import { useMutation, useQuery } from '@connectrpc/connect-query'
import { ArrowPathIcon, ViewfinderCircleIcon } from '@heroicons/react/20/solid'
import { Link } from 'react-router'
import { EmptyState } from '@/components/EmptyState'
import { Meter } from '@/components/Meter'
import { Stat } from '@/components/Stat'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { Callout, QueryError } from '@/components/ui/Callout'
import { Heading, Subheading } from '@/components/ui/Heading'
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
import {
  KeypointCheckKind,
  KeypointCheckStatus,
  LabelService,
  type KeypointCheck,
  type KeypointCheckType,
} from '@/gen/krill/v1/label_pb'
import { flash } from '@/lib/flash'
import { formatNumber, formatRelative, plural } from '@/lib/format'
import { invalidateService } from '@/lib/queryClient'
import { anyChecking, formatPixels, foundShare } from './keypointChecks'

const POLL = 3000
const muted = 'text-zinc-500 dark:text-zinc-400'

function boxes(n: number) {
  return plural(n, 'box', 'boxes')
}

function Result({ check }: { check: KeypointCheck }) {
  const compared = check.comparedCount > 0
  const tracking = check.kind === KeypointCheckKind.TRACKING
  return (
    <>
      <div className={`mt-6 grid gap-8 sm:grid-cols-4 ${tracking ? 'lg:grid-cols-5' : ''}`}>
        <Stat
          title={tracking ? 'Kept' : 'Found'}
          value={`${Math.round(foundShare(check) * 100)}%`}
          detail={
            <div className="flex items-center gap-3">
              <Meter
                label={tracking ? 'Boxes tracking kept keypoints on' : 'Boxes found'}
                value={foundShare(check)}
              />
              <span className="shrink-0 tabular-nums">
                {formatNumber(check.foundCount)}/{formatNumber(check.boxCount)}
              </span>
            </div>
          }
        />
        <Stat
          title="Median error"
          value={compared ? formatPixels(check.medianError) : '—'}
          detail="Each box's farthest keypoint"
        />
        <Stat
          title="90th percentile"
          value={compared ? formatPixels(check.p90Error) : '—'}
          detail={`Over ${boxes(check.comparedCount)}`}
        />
        {tracking ? (
          <>
            <Stat
              title="Lost"
              value={formatNumber(check.missedCount)}
              detail="Labeled by hand, left unlabeled by tracking"
            />
            <Stat
              title="Extra"
              value={formatNumber(check.extraCount)}
              detail="Placed where the labeler left none, as behind a wall"
            />
          </>
        ) : (
          <Stat
            title="Turned"
            value={formatNumber(check.turnedCount)}
            detail="Fit a quarter or half turn off"
          />
        )}
      </div>
      {compared && (
        <div className="mt-8 grid gap-x-10 gap-y-8 lg:grid-cols-2">
          <Table dense className="[--gutter:--spacing(6)] lg:[--gutter:--spacing(10)]">
            <TableHead>
              <TableRow>
                <TableHeader>Keypoint</TableHeader>
                <TableHeader className="text-right">Boxes</TableHeader>
                <TableHeader className="text-right">Median</TableHeader>
                <TableHeader className="text-right">90th percentile</TableHeader>
              </TableRow>
            </TableHead>
            <TableBody>
              {check.keypoints.map((k) => (
                <TableRow key={k.name}>
                  <TableCell className="font-medium">{k.name}</TableCell>
                  <TableCell className={`text-right tabular-nums ${muted}`}>
                    {formatNumber(k.count)}
                  </TableCell>
                  <TableCell className="text-right tabular-nums">
                    {formatPixels(k.median)}
                  </TableCell>
                  <TableCell className="text-right tabular-nums">{formatPixels(k.p90)}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
          <Table dense className="[--gutter:--spacing(6)] lg:[--gutter:--spacing(10)]">
            <TableHead>
              <TableRow>
                <TableHeader>Farthest off</TableHeader>
                {tracking && <TableHeader className="text-right">Frames in</TableHeader>}
                <TableHeader className="text-right">Error</TableHeader>
              </TableRow>
            </TableHead>
            <TableBody>
              {check.worst.map((b) => (
                <TableRow key={String(b.annotationId)}>
                  <TableCell>
                    <div className="flex items-center gap-2">
                      <Link
                        to={`/clips/${b.clipId}?frame=${b.frameIndex}`}
                        className="max-w-72 truncate font-medium text-zinc-950 hover:underline dark:text-white"
                      >
                        {b.videoName} · Clip {b.clipIndex + 1} · Frame {b.frameIndex + 1}
                      </Link>
                      {b.turned && <Badge color="amber">Turned</Badge>}
                    </div>
                  </TableCell>
                  {tracking && (
                    <TableCell className={`text-right tabular-nums ${muted}`}>
                      {formatNumber(b.framesTracked)}
                    </TableCell>
                  )}
                  <TableCell className="text-right tabular-nums">{formatPixels(b.error)}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}
    </>
  )
}

// One kind of check of a type: its latest result and the button that runs
// another.
function CheckBlock({
  type,
  kind,
  title,
  last,
  running,
  scope,
  disabled,
}: {
  type: KeypointCheckType
  kind: KeypointCheckKind
  title: string
  last?: KeypointCheck
  running?: KeypointCheck
  // What a check covers, or why it can't run.
  scope: string
  disabled: boolean
}) {
  const start = useMutation(LabelService.method.checkKeypoints, {
    onSuccess: () => invalidateService(LabelService),
    onError: (err) => flash.error('Could not start a check', err),
  })
  const finished = last?.finishedAt ? timestampDate(last.finishedAt) : null
  const tracking = kind === KeypointCheckKind.TRACKING
  return (
    <div className="mt-6">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <div>
          <h3 className="text-sm/6 font-semibold text-zinc-950 dark:text-white">{title}</h3>
          <p className={`text-sm/6 ${muted}`}>
            {scope}
            {finished && (
              <span title={finished.toLocaleString()}>
                {' '}
                · checked {formatRelative(finished)}
                {last?.createdBy && ` by ${last.createdBy.name}`}
              </span>
            )}
            {last?.status === KeypointCheckStatus.SUCCEEDED && tracking && (
              <>
                {' '}
                · {plural(last.trackCount, 'track', 'tracks')} over{' '}
                {plural(last.frameCount, 'frame', 'frames')}
              </>
            )}
          </p>
        </div>
        <Button
          outline
          disabled={start.isPending || !!running || disabled}
          onClick={() => start.mutate({ labelTypeId: type.labelTypeId, kind })}
        >
          <ArrowPathIcon data-slot="icon" className={running ? 'animate-spin' : ''} />
          {running ? `Checking ${boxes(running.boxCount)}` : 'Check'}
        </Button>
      </div>
      {last?.status === KeypointCheckStatus.FAILED && (
        <Callout title="Check failed" className="mt-4">
          {last.error}
        </Callout>
      )}
      {last?.status === KeypointCheckStatus.SUCCEEDED ? (
        <Result check={last} />
      ) : (
        !last &&
        !running &&
        !disabled && <p className={`mt-4 text-sm/6 ${muted}`}>Not checked yet</p>
      )}
    </div>
  )
}

function TypeCheck({ type }: { type: KeypointCheckType }) {
  return (
    <section className="mt-12">
      <Subheading className="flex items-center gap-2">
        <span className="size-2.5 rounded-full" style={{ backgroundColor: type.color }} />
        {type.title || type.name}
      </Subheading>
      <CheckBlock
        type={type}
        kind={KeypointCheckKind.FIT}
        title="Fit keypoints"
        last={type.last}
        running={type.running}
        scope={
          type.fittable
            ? `${boxes(type.posedBoxCount)} posed by hand`
            : 'Needs a quad covered by a reference design'
        }
        disabled={!type.fittable || type.posedBoxCount === 0}
      />
      <CheckBlock
        type={type}
        kind={KeypointCheckKind.TRACKING}
        title="Tracking"
        last={type.lastTracking}
        running={type.runningTracking}
        scope={`${plural(type.posedTrackCount, 'track', 'tracks')} posed by hand on two or more boxes`}
        disabled={type.posedTrackCount === 0}
      />
    </section>
  )
}

export function KeypointsPage() {
  const { data, isPending, error } = useQuery(
    LabelService.method.listKeypointChecks,
    {},
    { refetchInterval: (q) => (anyChecking(q.state.data?.types) ? POLL : false) },
  )
  return (
    <PageContentBlock title="Keypoints · Admin · Krill">
      <Heading>Keypoints</Heading>
      {data && (
        <Text className="mt-1 max-w-2xl">
          How far Fit keypoints and tracking land from keypoints labelers placed by hand, leaving
          the boxes as they are. A fit check fits up to {formatNumber(data.maxBoxes)} of a
          type&apos;s posed boxes, picked evenly across its videos. A tracking check tracks each
          track from its first posed box through its last without the labeler&apos;s keypoints in
          between, up to {formatNumber(data.maxTrackedFrames)} frames in all, and scores the posed
          boxes after the first.
        </Text>
      )}
      {isPending ? (
        <div className="flex justify-center py-16">
          <LoadingSpinner />
        </div>
      ) : error ? (
        <QueryError title="Could not load keypoint checks" error={error} className="mt-8" />
      ) : data.types.length === 0 ? (
        <div className="mt-10">
          <EmptyState
            icon={ViewfinderCircleIcon}
            title="No types to check"
            description="Label types show up here once they have keypoints."
          />
        </div>
      ) : (
        data.types.map((t) => <TypeCheck key={String(t.labelTypeId)} type={t} />)
      )}
    </PageContentBlock>
  )
}
