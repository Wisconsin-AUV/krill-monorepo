import { timestampDate } from '@bufbuild/protobuf/wkt'
import { useMutation, useQuery } from '@connectrpc/connect-query'
import { CubeIcon, PlayIcon } from '@heroicons/react/20/solid'
import { useState } from 'react'
import { EmptyState } from '@/components/EmptyState'
import { Meter } from '@/components/Meter'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { FormError, QueryError } from '@/components/ui/Callout'
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
import { TextButton, TextLink } from '@/components/ui/Text'
import {
  PipelineService,
  PrelabelSource,
  TrainingStatus,
  TrainingTrigger,
  type PrelabelRun,
  type RetrainSchedule,
  type TrainingRun,
} from '@/gen/krill/v1/pipeline_pb'
import { formatNumber, formatRelative, plural } from '@/lib/format'
import { invalidateService } from '@/lib/queryClient'
import {
  elapsed,
  percent,
  prelabelActive,
  prelabelStatus,
  runName,
  sizeNames,
  trainingActive,
  trainingStatus,
} from './pipeline'
import { TrainDialog } from './TrainDialog'

const POLL = 5000

function Card({
  title,
  action,
  children,
}: {
  title: string
  action?: React.ReactNode
  children: React.ReactNode
}) {
  return (
    <section className="flex flex-col rounded-xl p-6 ring-1 ring-zinc-950/10 dark:ring-white/10">
      <div className="flex items-center justify-between gap-4">
        <Subheading>{title}</Subheading>
        {action}
      </div>
      <div className="mt-4 flex-1 text-sm/6 text-zinc-500 dark:text-zinc-400">{children}</div>
    </section>
  )
}

export function RunStatus({ run }: { run: TrainingRun }) {
  const meta = trainingStatus[run.status]
  const epochs = run.config?.epochs ?? 0
  return (
    <span className="flex items-center gap-2">
      <Badge color={meta.color}>{meta.label}</Badge>
      {run.status === TrainingStatus.TRAINING && epochs > 0 && (
        <span className="text-zinc-500 tabular-nums dark:text-zinc-400">
          {run.epoch}/{epochs}
        </span>
      )}
      {run.pinned && <Badge color="sky">Pinned</Badge>}
    </span>
  )
}

function score(run: TrainingRun) {
  return run.test?.all ?? run.val?.all
}

function PinnedCard({ run }: { run: TrainingRun | undefined }) {
  if (!run) {
    return (
      <Card title="Pinned model">
        <p>None</p>
      </Card>
    )
  }
  const s = score(run)
  return (
    <Card title="Pinned model">
      <p className="text-2xl/8 font-semibold text-zinc-950 tabular-nums dark:text-white">
        <TextLink to={`/admin/models/${run.id}`}>{runName(run.id)}</TextLink>
      </p>
      <p className="mt-1">
        {s && `${percent(s.map5095)} mAP50-95 on ${run.test ? 'test' : 'val'} · `}
        {sizeNames[run.config?.size ?? 0]}
        {run.finishedAt && ` · ${formatRelative(timestampDate(run.finishedAt))}`}
      </p>
    </Card>
  )
}

function ScheduleCard({ schedule }: { schedule: RetrainSchedule }) {
  const off = schedule.hour < 0
  const ready = schedule.newLabels >= schedule.minNewLabels
  return (
    <Card title="Retraining" action={<TextLink to="/admin/settings">Settings</TextLink>}>
      <p className="text-2xl/8 font-semibold text-zinc-950 tabular-nums dark:text-white">
        {off ? 'Off' : `${String(schedule.hour).padStart(2, '0')}:00 ${schedule.timeZone}`}
      </p>
      <p className="mt-1 tabular-nums">
        {formatNumber(schedule.newLabels)} of {formatNumber(schedule.minNewLabels)} new labels
      </p>
      {!off && (
        <Meter
          label="New labels"
          value={schedule.minNewLabels > 0 ? schedule.newLabels / schedule.minNewLabels : 1}
          className="mt-3"
        />
      )}
      {!off && !ready && <p className="mt-2">Skips until enough new labels</p>}
    </Card>
  )
}

function PrelabelCard({
  latest,
  open,
  pinned,
}: {
  latest: PrelabelRun | undefined
  open: number
  pinned: boolean
}) {
  const start = useMutation(PipelineService.method.startPrelabel, {
    onSuccess: () => invalidateService(PipelineService),
  })
  const cancel = useMutation(PipelineService.method.cancelPrelabel, {
    onSuccess: () => invalidateService(PipelineService),
  })
  const active = latest !== undefined && prelabelActive(latest.status)
  const meta = latest && prelabelStatus[latest.status]
  return (
    <Card
      title="Pre-labels"
      action={
        active ? (
          <TextButton onClick={() => cancel.mutate({ id: latest.id })} disabled={cancel.isPending}>
            Cancel
          </TextButton>
        ) : (
          pinned && (
            <TextButton
              onClick={() => start.mutate({ source: PrelabelSource.YOLO })}
              disabled={start.isPending}
            >
              Run again
            </TextButton>
          )
        )
      }
    >
      <p className="text-2xl/8 font-semibold text-zinc-950 tabular-nums dark:text-white">
        {formatNumber(open)}
        <span className="ml-2 text-sm/6 font-normal text-zinc-500 dark:text-zinc-400">
          to check
        </span>
      </p>
      {latest && meta && (
        <>
          <p className="mt-1 flex items-center gap-2">
            <Badge color={meta.color}>{meta.label}</Badge>
            <span className="tabular-nums">
              {latest.clipsDone}/{latest.clipsTotal} clips ·{' '}
              {plural(latest.proposals, 'box', 'boxes')}
            </span>
          </p>
          {active && latest.clipsTotal > 0 && (
            <Meter
              label="Pre-labeling"
              value={latest.clipsDone / latest.clipsTotal}
              className="mt-3"
            />
          )}
          {latest.error && <p className="mt-2 text-red-600 dark:text-red-400">{latest.error}</p>}
        </>
      )}
      <FormError error={start.error ?? cancel.error} className="mt-3" />
    </Card>
  )
}

function RunsTable({ runs, now }: { runs: TrainingRun[]; now: number }) {
  return (
    <Table className="mt-4 [--gutter:--spacing(6)] lg:[--gutter:--spacing(10)]">
      <TableHead>
        <TableRow>
          <TableHeader>Run</TableHeader>
          <TableHeader>Status</TableHeader>
          <TableHeader>Started by</TableHeader>
          <TableHeader>Model</TableHeader>
          <TableHeader className="text-right">Images</TableHeader>
          <TableHeader className="text-right">Val mAP50-95</TableHeader>
          <TableHeader className="text-right">Test mAP50-95</TableHeader>
          <TableHeader>Queued</TableHeader>
          <TableHeader className="text-right">Took</TableHeader>
        </TableRow>
      </TableHead>
      <TableBody>
        {runs.map((r) => {
          const d = r.dataset
          return (
            <TableRow key={String(r.id)} href={`/admin/models/${r.id}`} title={runName(r.id)}>
              <TableCell className="font-medium">{runName(r.id)}</TableCell>
              <TableCell>
                <RunStatus run={r} />
              </TableCell>
              <TableCell className="text-zinc-500 dark:text-zinc-400">
                {r.trigger === TrainingTrigger.SCHEDULE ? 'Schedule' : r.createdBy || '—'}
              </TableCell>
              <TableCell className="text-zinc-500 dark:text-zinc-400">
                {sizeNames[r.config?.size ?? 0]} · {r.config?.epochs} epochs · {r.config?.imageSize}
                px
              </TableCell>
              <TableCell className="text-right tabular-nums">
                {d && d.trainImages + d.valImages > 0
                  ? formatNumber(d.trainImages + d.valImages + d.testImages)
                  : '—'}
              </TableCell>
              <TableCell className="text-right tabular-nums">
                {percent(r.val?.all?.map5095)}
              </TableCell>
              <TableCell className="text-right tabular-nums">
                {percent(r.test?.all?.map5095)}
              </TableCell>
              <TableCell className="text-zinc-500 dark:text-zinc-400">
                {r.createdAt && formatRelative(timestampDate(r.createdAt))}
              </TableCell>
              <TableCell className="text-right text-zinc-500 tabular-nums dark:text-zinc-400">
                {elapsed(r.startedAt, r.finishedAt, now) ?? '—'}
              </TableCell>
            </TableRow>
          )
        })}
      </TableBody>
    </Table>
  )
}

export function ModelsPage() {
  const [training, setTraining] = useState(false)
  const runs = useQuery(
    PipelineService.method.listTrainingRuns,
    {},
    {
      refetchInterval: (q) => (q.state.data?.runs.some(trainingActive) ? POLL : false),
    },
  )
  const prelabels = useQuery(
    PipelineService.method.listPrelabelRuns,
    {},
    {
      refetchInterval: (q) =>
        q.state.data?.runs.some((r) => prelabelActive(r.status)) ? POLL : false,
    },
  )
  const data = runs.data
  const active = data?.runs.some(trainingActive) ?? false
  const pinned = data?.runs.find((r) => r.pinned)
  const latestYolo = prelabels.data?.runs.find((r) => r.source === PrelabelSource.YOLO)

  return (
    <PageContentBlock title="Models · Admin · Krill">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <Heading>Models</Heading>
        <Button color="sky" onClick={() => setTraining(true)} disabled={!data || active}>
          <PlayIcon data-slot="icon" />
          Train now
        </Button>
      </div>
      {runs.error ? (
        <QueryError title="Could not load runs" error={runs.error} className="mt-8" />
      ) : !data ? (
        <div className="flex justify-center py-16">
          <LoadingSpinner />
        </div>
      ) : (
        <>
          <div className="mt-8 grid gap-6 lg:grid-cols-3">
            <PinnedCard run={pinned} />
            {data.schedule && <ScheduleCard schedule={data.schedule} />}
            <PrelabelCard
              latest={latestYolo}
              open={prelabels.data?.openYoloProposals ?? 0}
              pinned={pinned !== undefined}
            />
          </div>
          <Subheading className="mt-12">Runs</Subheading>
          {data.runs.length === 0 ? (
            <div className="mt-4">
              <EmptyState
                icon={CubeIcon}
                title="No runs yet"
                description="Train on verified labels."
              />
            </div>
          ) : (
            <RunsTable runs={data.runs} now={runs.dataUpdatedAt} />
          )}
        </>
      )}
      <TrainDialog
        open={training}
        onClose={() => setTraining(false)}
        defaults={data?.defaults}
        runs={data?.runs ?? []}
      />
    </PageContentBlock>
  )
}
