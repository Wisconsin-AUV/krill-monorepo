import { useQuery } from '@connectrpc/connect-query'
import { clsx } from 'clsx'
import { Link } from 'react-router'
import { Badge } from '@/components/ui/Badge'
import { QueryError } from '@/components/ui/Callout'
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
import {
  type CoverageCell,
  type CoverageLabelType,
  type CoverageVideo,
  InsightService,
} from '@/gen/krill/v1/insight_pb'
import { formatNumber } from '@/lib/format'
import { SortHeader } from './SortHeader'
import { type SortValue, useSort } from './useSort'

type TypeColumn = 'name' | 'verified' | 'proposed' | 'frames' | 'videos' | 'heldOut'

function typeValue(t: CoverageLabelType, key: TypeColumn): SortValue {
  switch (key) {
    case 'name':
      return t.name
    case 'verified':
      return Number(t.verified)
    case 'proposed':
      return Number(t.proposed)
    case 'frames':
      return Number(t.frames)
    case 'videos':
      return t.videos
    case 'heldOut':
      return Number(t.heldOutVerified)
  }
}

function Swatch({ color }: { color: string }) {
  return <span className="size-2.5 shrink-0 rounded-full" style={{ backgroundColor: color }} />
}

function LabelTypes({ types }: { types: CoverageLabelType[] }) {
  const { sorted, sort, toggle } = useSort<CoverageLabelType, TypeColumn>(types, typeValue, {
    key: 'verified',
    desc: false,
  })
  const header = (label: string, column: TypeColumn) => (
    <SortHeader
      label={label}
      column={column}
      sort={sort}
      onSort={toggle}
      className={column === 'name' ? undefined : 'text-right'}
    />
  )
  return (
    <Table className="mt-4 [--gutter:--spacing(6)] lg:[--gutter:--spacing(10)]">
      <TableHead>
        <TableRow>
          {header('Label', 'name')}
          {header('Verified boxes', 'verified')}
          {header('Proposed', 'proposed')}
          {header('Frames', 'frames')}
          {header('Videos', 'videos')}
          {header('Held out', 'heldOut')}
        </TableRow>
      </TableHead>
      <TableBody>
        {sorted.map((t) => (
          <TableRow
            key={String(t.id)}
            className={clsx((t.fewBoxes || t.fewVideos) && 'bg-amber-500/5 dark:bg-amber-400/5')}
          >
            <TableCell>
              <div className="flex items-center gap-2">
                <Swatch color={t.color} />
                <span className="font-medium text-zinc-950 dark:text-white">{t.name}</span>
                {t.fewBoxes && <Badge color="amber">Few boxes</Badge>}
                {t.fewVideos && <Badge color="amber">Few videos</Badge>}
              </div>
            </TableCell>
            <TableCell className="text-right tabular-nums">{formatNumber(t.verified)}</TableCell>
            <TableCell className="text-right text-zinc-500 tabular-nums dark:text-zinc-400">
              {formatNumber(t.proposed)}
            </TableCell>
            <TableCell className="text-right tabular-nums">{formatNumber(t.frames)}</TableCell>
            <TableCell className="text-right tabular-nums">{t.videos}</TableCell>
            <TableCell className="text-right text-zinc-500 tabular-nums dark:text-zinc-400">
              {t.heldOutVerified > 0n
                ? `${formatNumber(t.heldOutVerified)} in ${t.heldOutVideos}`
                : '—'}
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  )
}

function Matrix({
  videos,
  types,
  cells,
}: {
  videos: CoverageVideo[]
  types: CoverageLabelType[]
  cells: CoverageCell[]
}) {
  const at = new Map(cells.map((c) => [`${c.videoId}:${c.labelTypeId}`, c]))
  const max = Math.max(1, ...cells.map((c) => Number(c.verified)))
  return (
    <Table dense className="mt-4 [--gutter:--spacing(6)] lg:[--gutter:--spacing(10)]">
      <TableHead>
        <TableRow>
          <TableHeader>Video</TableHeader>
          <TableHeader className="text-right">Labeled</TableHeader>
          <TableHeader className="text-right">Empty</TableHeader>
          <TableHeader className="text-right">Unlabeled</TableHeader>
          {types.map((t) => (
            <TableHeader key={String(t.id)} className="text-right">
              <span className="inline-flex items-center gap-1.5">
                <Swatch color={t.color} />
                {t.name}
              </span>
            </TableHeader>
          ))}
        </TableRow>
      </TableHead>
      <TableBody>
        {videos.map((v) => (
          <TableRow key={String(v.id)} className={clsx(v.heldOut && 'opacity-60')}>
            <TableCell>
              <div className="flex items-center gap-2">
                <Link
                  to={`/videos/${v.id}`}
                  className="max-w-64 truncate font-medium text-zinc-950 hover:underline dark:text-white"
                >
                  {v.name}
                </Link>
                {v.heldOut && <Badge>Test</Badge>}
              </div>
            </TableCell>
            <TableCell className="text-right tabular-nums">{formatNumber(v.labeled)}</TableCell>
            <TableCell className="text-right tabular-nums">{formatNumber(v.empty)}</TableCell>
            <TableCell className="text-right text-zinc-500 tabular-nums dark:text-zinc-400">
              {formatNumber(v.unlabeled)}
            </TableCell>
            {types.map((t) => {
              const c = at.get(`${v.id}:${t.id}`)
              const verified = Number(c?.verified ?? 0)
              return (
                <TableCell
                  key={String(t.id)}
                  className="text-right tabular-nums"
                  title={c ? `${formatNumber(c.proposed)} proposed` : undefined}
                  style={
                    verified > 0
                      ? {
                          backgroundColor: `color-mix(in oklab, ${t.color} ${Math.round(
                            8 + (verified / max) * 32,
                          )}%, transparent)`,
                        }
                      : undefined
                  }
                >
                  {verified > 0 ? formatNumber(verified) : c && c.proposed > 0n ? '·' : ''}
                </TableCell>
              )
            })}
          </TableRow>
        ))}
      </TableBody>
    </Table>
  )
}

export function CoveragePage() {
  const { data, isPending, error } = useQuery(InsightService.method.getCoverage, {})
  return (
    <PageContentBlock title="Coverage · Admin · Krill">
      <Heading>Coverage</Heading>
      {isPending ? (
        <div className="flex justify-center py-16">
          <LoadingSpinner />
        </div>
      ) : error ? (
        <QueryError title="Could not load coverage" error={error} className="mt-8" />
      ) : (
        <>
          <div className="mt-8 flex flex-wrap items-baseline justify-between gap-2">
            <Subheading>Labels</Subheading>
            <span className="text-sm/6 text-zinc-500 dark:text-zinc-400">
              Thin below {formatNumber(data.minBoxes)} boxes or {data.minVideos} videos
            </span>
          </div>
          <LabelTypes types={data.labelTypes} />
          <Subheading className="mt-12">Videos</Subheading>
          <Matrix videos={data.videos} types={data.labelTypes} cells={data.cells} />
        </>
      )}
    </PageContentBlock>
  )
}
