import { timestampDate } from '@bufbuild/protobuf/wkt'
import { useMutation, useQuery } from '@connectrpc/connect-query'
import { ArrowDownTrayIcon, BookmarkIcon, StopIcon } from '@heroicons/react/20/solid'
import { clsx } from 'clsx'
import { useState } from 'react'
import { useParams } from 'react-router'
import { ConfirmAlert } from '@/components/ConfirmAlert'
import { Meter } from '@/components/Meter'
import { SegmentedTabs } from '@/components/SegmentedTabs'
import { Button } from '@/components/ui/Button'
import { Callout, QueryError } from '@/components/ui/Callout'
import {
  DescriptionDetails,
  DescriptionList,
  DescriptionTerm,
} from '@/components/ui/DescriptionList'
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
import { TextLink } from '@/components/ui/Text'
import {
  PipelineService,
  TrainingStatus,
  TrainingTrigger,
  type ClassMetrics,
  type EvalMetrics,
  type TrainingRun,
} from '@/gen/krill/v1/pipeline_pb'
import { flash } from '@/lib/flash'
import { formatBytes, formatNumber } from '@/lib/format'
import { invalidateService } from '@/lib/queryClient'
import { EpochChart } from './EpochChart'
import { RunStatus } from './ModelsPage'
import { RunTrials } from './RunTrials'
import { elapsed, percent, runName, sizeNames, trainingActive } from './pipeline'

const POLL = 5000
const fullDate = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' })

type Split = 'test' | 'val'

function Delta({ value, baseline }: { value: number; baseline: number | undefined }) {
  if (baseline === undefined) return <span className="text-zinc-400">—</span>
  const d = (value - baseline) * 100
  return (
    <span
      className={clsx(
        'tabular-nums',
        Math.abs(d) < 0.05
          ? 'text-zinc-500 dark:text-zinc-400'
          : d > 0
            ? 'text-green-600 dark:text-green-400'
            : 'text-red-600 dark:text-red-400',
      )}
    >
      {d > 0 ? '+' : ''}
      {d.toFixed(1)}
    </span>
  )
}

function MetricsTable({
  metrics,
  baseline,
  baselineLabel,
}: {
  metrics: EvalMetrics
  baseline: EvalMetrics | undefined
  baselineLabel: string | null
}) {
  const byName = new Map(baseline?.classes.map((c) => [c.name, c]))
  const rows: [ClassMetrics, ClassMetrics | undefined][] = [
    ...(metrics.all
      ? [[metrics.all, baseline?.all] as [ClassMetrics, ClassMetrics | undefined]]
      : []),
    ...metrics.classes.map(
      (c) => [c, byName.get(c.name)] as [ClassMetrics, ClassMetrics | undefined],
    ),
  ]
  return (
    <Table dense className="mt-4 [--gutter:--spacing(6)] lg:[--gutter:--spacing(10)]">
      <TableHead>
        <TableRow>
          <TableHeader>Class</TableHeader>
          <TableHeader className="text-right">Images</TableHeader>
          <TableHeader className="text-right">Boxes</TableHeader>
          <TableHeader className="text-right">Precision</TableHeader>
          <TableHeader className="text-right">Recall</TableHeader>
          <TableHeader className="text-right">mAP50</TableHeader>
          <TableHeader className="text-right">mAP50-95</TableHeader>
          {baselineLabel && <TableHeader className="text-right">vs {baselineLabel}</TableHeader>}
        </TableRow>
      </TableHead>
      <TableBody>
        {rows.map(([c, b], i) => (
          <TableRow key={c.name} className={i === 0 ? 'font-medium' : undefined}>
            <TableCell>{i === 0 ? 'All classes' : c.name}</TableCell>
            <TableCell className="text-right tabular-nums">
              {c.images ? formatNumber(c.images) : '—'}
            </TableCell>
            <TableCell className="text-right tabular-nums">{formatNumber(c.instances)}</TableCell>
            <TableCell className="text-right tabular-nums">{percent(c.precision)}</TableCell>
            <TableCell className="text-right tabular-nums">{percent(c.recall)}</TableCell>
            <TableCell className="text-right tabular-nums">{percent(c.map50)}</TableCell>
            <TableCell className="text-right tabular-nums">{percent(c.map5095)}</TableCell>
            {baselineLabel && (
              <TableCell className="text-right">
                <Delta value={c.map5095} baseline={b?.map5095} />
              </TableCell>
            )}
          </TableRow>
        ))}
      </TableBody>
    </Table>
  )
}

function Metrics({ run, pinned }: { run: TrainingRun; pinned: TrainingRun | undefined }) {
  const splits: { key: Split; label: string }[] = [
    ...(run.test ? [{ key: 'test' as const, label: 'Held-out test' }] : []),
    ...(run.val ? [{ key: 'val' as const, label: 'Validation' }] : []),
  ]
  const [split, setSplit] = useState<Split>(splits[0]?.key ?? 'val')
  if (splits.length === 0) return null
  const metrics = split === 'test' ? run.test : run.val
  // The pinned model scored on these same images beats its own scores from
  // when it trained, since the test set may have grown since.
  const same = split === 'test' ? run.baselineTest : run.baselineVal
  const own = pinned && (split === 'test' ? pinned.test : pinned.val)
  const baseline = same ?? own
  const baselineId = same ? run.baselineRunId : pinned?.id
  return (
    <section className="mt-12">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <Subheading>Scores</Subheading>
        {splits.length > 1 && (
          <SegmentedTabs options={splits} value={split} onChange={(k) => setSplit(k as Split)} />
        )}
      </div>
      {metrics && (
        <MetricsTable
          metrics={metrics}
          baseline={baseline}
          baselineLabel={baseline && baselineId ? `${runName(baselineId)}${same ? '' : '*'}` : null}
        />
      )}
      {baseline && !same && (
        <p className="mt-3 text-sm/6 text-zinc-500 dark:text-zinc-400">
          * Scored on its own {split} images
        </p>
      )}
    </section>
  )
}

function Details({ run, now }: { run: TrainingRun; now: number }) {
  const c = run.config
  const d = run.dataset
  const o = c?.dataset
  const items: [string, React.ReactNode][] = [
    ['Started by', run.trigger === TrainingTrigger.SCHEDULE ? 'Schedule' : run.createdBy || '—'],
    ['Model', sizeNames[c?.size ?? 0]],
    [
      'Start from',
      c?.baseRunId ? (
        <TextLink key="base" to={`/admin/models/${c.baseRunId}`}>
          {runName(c.baseRunId)}
        </TextLink>
      ) : (
        'COCO weights'
      ),
    ],
    ['Epochs', c?.epochs],
    ['Image size', c && `${c.imageSize}px`],
    ['Batch size', c?.batch],
    [
      'Images',
      d && d.trainImages + d.valImages > 0
        ? `${formatNumber(d.trainImages)} train · ${formatNumber(d.valImages)} val · ${formatNumber(d.testImages)} test`
        : '—',
    ],
    [
      'Boxes',
      d && d.trainBoxes + d.valBoxes > 0
        ? `${formatNumber(d.trainBoxes)} train · ${formatNumber(d.valBoxes)} val · ${formatNumber(d.testBoxes)} test`
        : '—',
    ],
    [
      'Dataset',
      o &&
        [
          `${Math.round(o.valFraction * 100)}% val`,
          o.stride > 1 ? `every ${o.stride} frames` : 'every frame',
          o.dedup ? `dedup ≤${o.dedupDistance}` : 'no dedup',
        ].join(' · '),
    ],
    ['Verified boxes', formatNumber(run.verifiedBoxes)],
    ['GPU', run.device || '—'],
    ['Queued', run.createdAt && fullDate.format(timestampDate(run.createdAt))],
    ['Took', elapsed(run.startedAt, run.finishedAt, now) ?? '—'],
    ['Weights', run.weightsBytes > 0n ? formatBytes(Number(run.weightsBytes)) : '—'],
  ]
  return (
    <DescriptionList className="mt-8">
      {items.map(([term, details]) => (
        <div key={term} className="contents">
          <DescriptionTerm>{term}</DescriptionTerm>
          <DescriptionDetails>{details ?? '—'}</DescriptionDetails>
        </div>
      ))}
    </DescriptionList>
  )
}

export function ModelRunPage() {
  const { id = '' } = useParams()
  const runId = /^\d+$/.test(id) ? BigInt(id) : 0n
  const [confirm, setConfirm] = useState<'pin' | 'cancel' | null>(null)
  const { data, isPending, error, dataUpdatedAt } = useQuery(
    PipelineService.method.getTrainingRun,
    { id: runId },
    {
      enabled: runId > 0n,
      refetchInterval: (q) =>
        q.state.data?.run && trainingActive(q.state.data.run) ? POLL : false,
    },
  )
  const done = () => {
    setConfirm(null)
    return invalidateService(PipelineService)
  }
  const pin = useMutation(PipelineService.method.pinModel, {
    onSuccess: async () => {
      await done()
      flash.success('Model pinned', 'Pre-labeling with it has started.')
    },
  })
  const cancel = useMutation(PipelineService.method.cancelTraining, { onSuccess: done })
  const run = data?.run

  return (
    <PageContentBlock title={`${runName(runId)} · Models · Admin · Krill`}>
      <TextLink to="/admin/models" className="text-sm/6">
        Models
      </TextLink>
      {isPending && runId > 0n ? (
        <div className="flex justify-center py-16">
          <LoadingSpinner />
        </div>
      ) : error || !run ? (
        <QueryError title="Could not load the run" error={error ?? 'Not found'} className="mt-8" />
      ) : (
        <>
          <div className="mt-2 flex flex-wrap items-center justify-between gap-4">
            <div className="flex flex-wrap items-center gap-3">
              <Heading>{runName(run.id)}</Heading>
              <RunStatus run={run} />
            </div>
            <div className="flex gap-3">
              {trainingActive(run) && (
                <Button outline onClick={() => setConfirm('cancel')}>
                  <StopIcon data-slot="icon" />
                  Cancel
                </Button>
              )}
              {run.weightsUrl && (
                <Button outline to={run.weightsUrl}>
                  <ArrowDownTrayIcon data-slot="icon" />
                  Weights
                </Button>
              )}
              {run.status === TrainingStatus.SUCCEEDED && !run.pinned && (
                <Button color="sky" onClick={() => setConfirm('pin')}>
                  <BookmarkIcon data-slot="icon" />
                  Pin
                </Button>
              )}
            </div>
          </div>
          {run.error && (
            <Callout title="Training failed" className="mt-6">
              {run.error}
            </Callout>
          )}
          {run.status === TrainingStatus.TRAINING && run.config && (
            <Meter label="Epochs done" value={run.epoch / run.config.epochs} className="mt-6" />
          )}
          {run.history.length > 0 && (
            <section className="mt-10">
              <EpochChart history={run.history} epochs={run.config?.epochs ?? run.history.length} />
            </section>
          )}
          <Metrics run={run} pinned={data.pinned} />
          {run.status === TrainingStatus.SUCCEEDED && <RunTrials runId={run.id} />}
          <Details run={run} now={dataUpdatedAt} />
          {run.log && (
            <section className="mt-12">
              <Subheading>Log</Subheading>
              <pre className="mt-4 max-h-96 overflow-auto rounded-lg bg-zinc-950/5 p-4 font-mono text-xs/5 text-zinc-700 dark:bg-white/5 dark:text-zinc-300">
                {run.log}
              </pre>
            </section>
          )}
          <ConfirmAlert
            open={confirm === 'pin'}
            onClose={() => setConfirm(null)}
            onConfirm={() => pin.mutate({ id: run.id })}
            title={`Pin ${runName(run.id)}?`}
            description="Its boxes replace the open YOLO proposals on unlabeled frames."
            confirmLabel="Pin"
            busy={pin.isPending}
            error={pin.error}
          />
          <ConfirmAlert
            open={confirm === 'cancel'}
            onClose={() => setConfirm(null)}
            onConfirm={() => cancel.mutate({ id: run.id })}
            title={`Cancel ${runName(run.id)}?`}
            description="Training stops and nothing is saved."
            confirmLabel="Cancel run"
            busy={cancel.isPending}
            error={cancel.error}
          />
        </>
      )}
    </PageContentBlock>
  )
}
