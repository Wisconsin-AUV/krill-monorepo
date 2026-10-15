import { timestampDate } from '@bufbuild/protobuf/wkt'
import { useQuery } from '@connectrpc/connect-query'
import { useState } from 'react'
import { Link, useSearchParams } from 'react-router'
import { SegmentedTabs } from '@/components/SegmentedTabs'
import { Avatar } from '@/components/ui/Avatar'
import { Badge } from '@/components/ui/Badge'
import { QueryError } from '@/components/ui/Callout'
import { Dialog, DialogBody, DialogTitle } from '@/components/ui/Dialog'
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
import { type GoldLabeler, GoldService } from '@/gen/krill/v1/gold_pb'
import {
  InsightService,
  type LabelerStats,
  type TrendPoint,
  TrendBucket,
} from '@/gen/krill/v1/insight_pb'
import type { Period } from '@/gen/krill/v1/stats_pb'
import { formatNumber } from '@/lib/format'
import { periodFromKey, periods, profilePath, timeZone } from '@/lib/stats'
import { Sparkline } from './Sparkline'
import { SortHeader } from './SortHeader'
import { type SortValue, useSort } from './useSort'

const pct = (n: number) => `${Math.round(n * 100)}%`

function formatActive(seconds: number): string {
  const m = Math.round(seconds / 60)
  if (seconds < 60) return `${Math.round(seconds)}s`
  if (m < 60) return `${m}m`
  return `${Math.floor(m / 60)}h ${m % 60}m`
}

function proposals(l: LabelerStats) {
  let accepted = 0
  let edited = 0
  let rejected = 0
  for (const p of l.proposals) {
    accepted += Number(p.accepted)
    edited += Number(p.edited)
    rejected += Number(p.rejected)
  }
  return { accepted, edited, rejected, total: accepted + edited + rejected }
}

type Row = { stats: LabelerStats; gold?: GoldLabeler }

type Column =
  | 'name'
  | 'frames'
  | 'boxes'
  | 'kept'
  | 'acceptRate'
  | 'rejectRate'
  | 'corrected'
  | 'active'
  | 'perFrame'
  | 'gold'

function ratio(n: number, d: number) {
  return d > 0 ? n / d : undefined
}

function value({ stats: l, gold }: Row, key: Column): SortValue {
  const p = proposals(l)
  const frames = Number(l.framesLabeled + l.framesEmpty)
  switch (key) {
    case 'name':
      return l.user?.name ?? ''
    case 'frames':
      return frames
    case 'boxes':
      return Number(l.boxesDrawn)
    case 'kept':
      return p.accepted + p.edited
    case 'acceptRate':
      return ratio(p.accepted, p.total)
    case 'rejectRate':
      return ratio(p.rejected, p.total)
    case 'corrected':
      return ratio(Number(l.framesCorrected), Number(l.framesMarked))
    case 'active':
      return l.activeSeconds
    case 'perFrame':
      return ratio(l.activeSeconds, frames)
    case 'gold':
      return gold && gold.attempts > 0 ? gold.accuracy : undefined
  }
}

const dash = <span className="text-zinc-400 dark:text-zinc-500">—</span>

function show(v: SortValue, format: (n: number) => string) {
  return typeof v === 'number' ? format(v) : dash
}

const columns: { key: Column; label: string; format: (n: number) => string }[] = [
  { key: 'frames', label: 'Frames', format: formatNumber },
  { key: 'boxes', label: 'Drawn', format: formatNumber },
  { key: 'kept', label: 'Kept', format: formatNumber },
  { key: 'acceptRate', label: 'Accepted', format: pct },
  { key: 'rejectRate', label: 'Rejected', format: pct },
  { key: 'corrected', label: 'Corrected', format: pct },
  { key: 'active', label: 'Active', format: formatActive },
  { key: 'perFrame', label: 'Per frame', format: formatActive },
  { key: 'gold', label: 'Gold', format: pct },
]

export function LabelersPage() {
  const [params, setParams] = useSearchParams()
  const period = periodFromKey(params.get('period'))
  const [shown, setShown] = useState<Row | null>(null)
  const { data, isPending, error } = useQuery(InsightService.method.getLabelerStats, {
    period: period.period,
    timeZone,
  })
  const gold = useQuery(GoldService.method.getGoldStats, {})
  const goldBy = new Map(gold.data?.labelers.map((g) => [g.user?.id, g]))
  const rows: Row[] = (data?.labelers ?? []).map((stats) => ({
    stats,
    gold: goldBy.get(stats.user?.id),
  }))
  const { sorted, sort, toggle } = useSort<Row, Column>(rows, value, { key: 'frames', desc: true })

  return (
    <PageContentBlock title="Labelers · Admin · Krill">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <Heading>Labelers</Heading>
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
        <QueryError title="Could not load labelers" error={error} className="mt-8" />
      ) : rows.length === 0 ? (
        <p className="py-10 text-center text-sm/6 text-zinc-500 dark:text-zinc-400">
          No labeling {period.label.toLowerCase()}
        </p>
      ) : (
        <Table className="mt-8 [--gutter:--spacing(6)] lg:[--gutter:--spacing(10)]">
          <TableHead>
            <TableRow>
              <SortHeader label="Labeler" column="name" sort={sort} onSort={toggle} />
              {columns.map((c) => (
                <SortHeader
                  key={c.key}
                  label={c.label}
                  column={c.key}
                  sort={sort}
                  onSort={toggle}
                  className="text-right"
                />
              ))}
            </TableRow>
          </TableHead>
          <TableBody>
            {sorted.map((row) => (
              <TableRow
                key={row.stats.user?.id}
                className="cursor-pointer hover:bg-zinc-950/[2.5%] dark:hover:bg-white/[2.5%]"
                onClick={() => setShown(row)}
              >
                <TableCell>
                  <div className="flex items-center gap-3">
                    <Avatar seed={row.stats.user?.id ?? ''} className="size-6" />
                    <span className="font-medium text-zinc-950 dark:text-white">
                      {row.stats.user?.name}
                    </span>
                    {row.gold?.needsReview && <Badge color="amber">In review</Badge>}
                  </div>
                </TableCell>
                {columns.map((c) => (
                  <TableCell key={c.key} className="text-right tabular-nums">
                    {show(value(row, c.key), c.format)}
                  </TableCell>
                ))}
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}
      <Dialog size="3xl" open={shown !== null} onClose={() => setShown(null)}>
        {shown && <Labeler row={shown} period={period.period} />}
      </Dialog>
    </PageContentBlock>
  )
}

const bucketFormat = {
  [TrendBucket.HOUR]: new Intl.DateTimeFormat(undefined, { hour: 'numeric' }),
  [TrendBucket.DAY]: new Intl.DateTimeFormat(undefined, {
    weekday: 'short',
    day: 'numeric',
    month: 'short',
  }),
  [TrendBucket.WEEK]: new Intl.DateTimeFormat(undefined, {
    day: 'numeric',
    month: 'short',
    year: 'numeric',
  }),
} as Record<TrendBucket, Intl.DateTimeFormat>

const correctionLabels: Record<string, string> = {
  added: 'Missed boxes added',
  moved: 'Boxes moved',
  deleted: 'Boxes deleted',
  relabeled: 'Boxes relabeled',
}

function Trend({
  label,
  points,
  pick,
  format,
  pointLabel,
}: {
  label: string
  points: TrendPoint[]
  pick: (p: TrendPoint) => number
  format: (n: number) => string
  pointLabel: (i: number) => string
}) {
  const values = points.map(pick)
  const total = values.reduce((a, b) => a + b, 0)
  return (
    <div>
      <div className="flex items-baseline justify-between text-sm/6">
        <span className="text-zinc-500 dark:text-zinc-400">{label}</span>
        <span className="font-medium tabular-nums">{format(total)}</span>
      </div>
      <div className="mt-2">
        <Sparkline
          label={label}
          values={values}
          max={Math.max(...values, 1)}
          format={format}
          interval={0}
          pointLabel={pointLabel}
        />
      </div>
    </div>
  )
}

function Labeler({ row, period }: { row: Row; period: Period }) {
  const { stats: l, gold } = row
  const { data, isPending, error } = useQuery(InsightService.method.getLabelerTrend, {
    userId: l.user?.id ?? '',
    period,
    timeZone,
  })
  const points = data?.points ?? []
  const fmt = data ? bucketFormat[data.bucket] : undefined
  const pointLabel = (i: number) => {
    const start = points[i]?.start
    return start && fmt ? fmt.format(timestampDate(start)) : ''
  }

  return (
    <>
      <DialogTitle>
        <span className="flex items-center gap-3">
          <Avatar seed={l.user?.id ?? ''} className="size-8" />
          <Link to={profilePath(l.user?.username ?? '')} className="hover:underline">
            {l.user?.name}
          </Link>
        </span>
      </DialogTitle>
      <DialogBody>
        {isPending ? (
          <div className="flex justify-center py-10">
            <LoadingSpinner />
          </div>
        ) : error ? (
          <QueryError title="Could not load the trend" error={error} />
        ) : (
          <div className="grid gap-6 sm:grid-cols-2">
            <Trend
              label="Frames"
              points={points}
              pick={(p) => Number(p.frames)}
              format={formatNumber}
              pointLabel={pointLabel}
            />
            <Trend
              label="Active"
              points={points}
              pick={(p) => p.activeSeconds}
              format={formatActive}
              pointLabel={pointLabel}
            />
            <Trend
              label="Boxes drawn"
              points={points}
              pick={(p) => Number(p.boxesDrawn)}
              format={formatNumber}
              pointLabel={pointLabel}
            />
            <Trend
              label="Frames corrected"
              points={points}
              pick={(p) => Number(p.framesCorrected)}
              format={formatNumber}
              pointLabel={pointLabel}
            />
          </div>
        )}

        <div className="mt-8 grid gap-8 sm:grid-cols-2">
          <div>
            <Subheading level={3}>Proposals</Subheading>
            {l.proposals.length === 0 ? (
              <Text className="mt-2">None</Text>
            ) : (
              <Table dense className="mt-2">
                <TableHead>
                  <TableRow>
                    <TableHeader>Source</TableHeader>
                    <TableHeader className="text-right">Kept</TableHeader>
                    <TableHeader className="text-right">Edited</TableHeader>
                    <TableHeader className="text-right">Rejected</TableHeader>
                  </TableRow>
                </TableHead>
                <TableBody>
                  {l.proposals.map((p) => (
                    <TableRow key={p.source}>
                      <TableCell className="uppercase">{p.source}</TableCell>
                      <TableCell className="text-right tabular-nums">
                        {formatNumber(p.accepted)}
                      </TableCell>
                      <TableCell className="text-right tabular-nums">
                        {formatNumber(p.edited)}
                      </TableCell>
                      <TableCell className="text-right tabular-nums">
                        {formatNumber(p.rejected)}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}
          </div>
          <div>
            <Subheading level={3}>Corrections</Subheading>
            {(data?.corrections ?? []).length === 0 ? (
              <Text className="mt-2">None</Text>
            ) : (
              <dl className="mt-2 space-y-1 text-sm/6">
                {data?.corrections.map((c) => (
                  <div key={c.kind} className="flex justify-between">
                    <dt className="text-zinc-500 dark:text-zinc-400">
                      {correctionLabels[c.kind] ?? c.kind}
                    </dt>
                    <dd className="tabular-nums">{formatNumber(c.boxes)}</dd>
                  </div>
                ))}
              </dl>
            )}
            <Subheading level={3} className="mt-6">
              Gold
            </Subheading>
            {gold && gold.attempts > 0 ? (
              <dl className="mt-2 space-y-1 text-sm/6">
                <div className="flex justify-between">
                  <dt className="text-zinc-500 dark:text-zinc-400">Accuracy</dt>
                  <dd className="tabular-nums">{pct(gold.accuracy)}</dd>
                </div>
                <div className="flex justify-between">
                  <dt className="text-zinc-500 dark:text-zinc-400">Mean IoU</dt>
                  <dd className="tabular-nums">{gold.meanIou.toFixed(2)}</dd>
                </div>
                <div className="flex justify-between">
                  <dt className="text-zinc-500 dark:text-zinc-400">Checks</dt>
                  <dd className="tabular-nums">
                    <Link to="/review/gold" className="hover:underline">
                      {formatNumber(gold.attempts)}
                    </Link>
                  </dd>
                </div>
              </dl>
            ) : (
              <Text className="mt-2">No checks</Text>
            )}
          </div>
        </div>
      </DialogBody>
    </>
  )
}
