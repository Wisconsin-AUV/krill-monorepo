import { timestampDate } from '@bufbuild/protobuf/wkt'
import { useInfiniteQuery, useMutation } from '@connectrpc/connect-query'
import {
  ArrowPathIcon,
  EllipsisHorizontalIcon,
  InformationCircleIcon,
  XMarkIcon,
} from '@heroicons/react/20/solid'
import { useState } from 'react'
import { useSearchParams } from 'react-router'
import { ConfirmAlert } from '@/components/ConfirmAlert'
import { Avatar } from '@/components/ui/Avatar'
import { Badge, type BadgeColor } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { QueryError } from '@/components/ui/Callout'
import { Dialog, DialogActions, DialogBody, DialogTitle } from '@/components/ui/Dialog'
import {
  DescriptionDetails,
  DescriptionList,
  DescriptionTerm,
} from '@/components/ui/DescriptionList'
import {
  Dropdown,
  DropdownButton,
  DropdownItem,
  DropdownLabel,
  DropdownMenu,
} from '@/components/ui/Dropdown'
import { Heading } from '@/components/ui/Heading'
import LoadingSpinner from '@/components/ui/LoadingSpinner'
import PageContentBlock from '@/components/ui/PageContentBlock'
import { Select } from '@/components/ui/Select'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/Table'
import { ComputeService, type Job, type JobCount, JobState } from '@/gen/krill/v1/compute_pb'
import { flash } from '@/lib/flash'
import { formatDuration, formatRelative } from '@/lib/format'
import { invalidateService } from '@/lib/queryClient'

const fullDate = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'medium' })

const jobKinds: Record<string, string> = {
  ingest: 'Ingest',
  embed: 'Embedding',
  track: 'Tracking',
  exemplar: 'Similar objects',
  export: 'Export',
  train: 'Training',
  prelabel: 'Pre-label',
  retrain_schedule: 'Scheduled retrain',
  backup: 'Backup',
  backup_schedule: 'Scheduled backup',
  storage_scan: 'Storage scan',
  storage_cleanup: 'Storage cleanup',
  purge_objects: 'Delete files',
  auto_purge_trash: 'Empty old trash',
}

const states: { state: JobState; param: string; label: string; color: BadgeColor }[] = [
  { state: JobState.QUEUED, param: 'queued', label: 'Queued', color: 'zinc' },
  { state: JobState.RUNNING, param: 'running', label: 'Running', color: 'sky' },
  { state: JobState.RETRYABLE, param: 'retrying', label: 'Retrying', color: 'amber' },
  { state: JobState.FAILED, param: 'failed', label: 'Failed', color: 'red' },
  { state: JobState.CANCELLED, param: 'cancelled', label: 'Cancelled', color: 'zinc' },
  { state: JobState.COMPLETED, param: 'completed', label: 'Completed', color: 'green' },
]
const stateInfo = (s: JobState) => states.find((x) => x.state === s)

const kindLabel = (kind: string) => jobKinds[kind] ?? kind

function date(t: Job['createdAt']) {
  return t ? timestampDate(t) : null
}

function runTime(job: Job, now: number) {
  const started = date(job.attemptedAt)
  if (!started || job.state === JobState.QUEUED) return null
  const end = date(job.finalizedAt)?.getTime() ?? now
  return formatDuration(end - started.getTime())
}

function StateBadge({ job }: { job: Job }) {
  const info = stateInfo(job.state)
  if (job.cancelling) return <Badge color="amber">Cancelling</Badge>
  if (job.gpu && !job.gpu.pickedAt) return <Badge color="zinc">Waiting for worker</Badge>
  return <Badge color={info?.color ?? 'zinc'}>{info?.label ?? 'Unknown'}</Badge>
}

const canRetry = (j: Job) =>
  j.state === JobState.FAILED || j.state === JobState.CANCELLED || j.state === JobState.RETRYABLE
const canCancel = (j: Job) =>
  (j.state === JobState.QUEUED || j.state === JobState.RUNNING || j.state === JobState.RETRYABLE) &&
  !j.cancelling
const notStarted = (j: Job) => j.state === JobState.QUEUED || j.state === JobState.RETRYABLE

function JobDialog({ job, onClose }: { job: Job | null; onClose: () => void }) {
  const created = job && date(job.createdAt)
  const scheduled = job && date(job.scheduledAt)
  const attempted = job && date(job.attemptedAt)
  const finalized = job && date(job.finalizedAt)
  return (
    <Dialog open={job !== null} onClose={onClose} size="2xl">
      <DialogTitle>
        {job && `${kindLabel(job.kind)} #${job.id}`}
        {job?.target && (
          <span className="ml-2 font-normal text-zinc-500 dark:text-zinc-400">{job.target}</span>
        )}
      </DialogTitle>
      <DialogBody>
        {job && (
          <>
            <DescriptionList>
              <DescriptionTerm>Queue</DescriptionTerm>
              <DescriptionDetails>{job.queue}</DescriptionDetails>
              <DescriptionTerm>Created</DescriptionTerm>
              <DescriptionDetails>{created && fullDate.format(created)}</DescriptionDetails>
              {scheduled && notStarted(job) && (
                <>
                  <DescriptionTerm>Scheduled</DescriptionTerm>
                  <DescriptionDetails>{fullDate.format(scheduled)}</DescriptionDetails>
                </>
              )}
              {attempted && (
                <>
                  <DescriptionTerm>Last attempt</DescriptionTerm>
                  <DescriptionDetails>{fullDate.format(attempted)}</DescriptionDetails>
                </>
              )}
              {finalized && (
                <>
                  <DescriptionTerm>Finished</DescriptionTerm>
                  <DescriptionDetails>{fullDate.format(finalized)}</DescriptionDetails>
                </>
              )}
              {job.gpu && (
                <>
                  <DescriptionTerm>GPU task</DescriptionTerm>
                  <DescriptionDetails>
                    #{String(job.gpu.id)}
                    {job.gpu.workerHostname && ` on ${job.gpu.workerHostname}`}
                  </DescriptionDetails>
                </>
              )}
              <DescriptionTerm>Arguments</DescriptionTerm>
              <DescriptionDetails>
                <code className="font-mono text-xs break-all">{job.args}</code>
              </DescriptionDetails>
            </DescriptionList>
            {job.errors.length > 0 && (
              <ol className="mt-6 space-y-3">
                {job.errors.map((e, i) => (
                  <li key={i} className="rounded-lg bg-red-500/5 p-3 text-sm/6 dark:bg-red-500/10">
                    <p className="text-zinc-500 dark:text-zinc-400">
                      Attempt {e.attempt}
                      {e.at && ` · ${fullDate.format(timestampDate(e.at))}`}
                    </p>
                    <p className="font-mono text-xs break-words whitespace-pre-wrap text-red-700 dark:text-red-400">
                      {e.message}
                    </p>
                  </li>
                ))}
              </ol>
            )}
          </>
        )}
      </DialogBody>
      <DialogActions>
        <Button plain onClick={onClose}>
          Close
        </Button>
      </DialogActions>
    </Dialog>
  )
}

function countFor(counts: JobCount[], kind: string, state: JobState) {
  return counts
    .filter((c) => (kind === '' || c.kind === kind) && c.state === state)
    .reduce((n, c) => n + c.count, 0)
}

export function JobsPage() {
  const [params, setParams] = useSearchParams()
  const kind = params.get('kind') ?? ''
  const stateParam = params.get('state') ?? ''
  const state = states.find((s) => s.param === stateParam)
  const [viewing, setViewing] = useState<Job | null>(null)
  const [cancelling, setCancelling] = useState<Job | null>(null)

  const setFilter = (key: string, value: string) => {
    const next = new URLSearchParams(params)
    if (value) next.set(key, value)
    else next.delete(key)
    setParams(next, { replace: true })
  }

  const { data, isPending, error, fetchNextPage, hasNextPage, isFetchingNextPage, dataUpdatedAt } =
    useInfiniteQuery(
      ComputeService.method.listJobs,
      { kinds: kind ? [kind] : [], states: state ? [state.state] : [], pageToken: '' },
      {
        pageParamKey: 'pageToken',
        getNextPageParam: (p) => p.nextPageToken || undefined,
        refetchInterval: 5000,
      },
    )
  const jobs = data?.pages.flatMap((p) => p.jobs) ?? []
  const counts = data?.pages[0]?.counts ?? []
  const kinds = [...new Set([...Object.keys(jobKinds), ...counts.map((c) => c.kind)])]

  const done = () => invalidateService(ComputeService)
  const retry = useMutation(ComputeService.method.retryJob, {
    onSuccess: done,
    onError: (err) => flash.error('Could not retry job', err),
  })
  const cancel = useMutation(ComputeService.method.cancelJob, {
    onSuccess: async () => {
      await done()
      setCancelling(null)
    },
  })
  const prioritize = useMutation(ComputeService.method.setJobPriority, {
    onSuccess: done,
    onError: (err) => flash.error('Could not change priority', err),
  })

  return (
    <PageContentBlock title="Jobs · Admin · Krill">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <Heading>Jobs</Heading>
        <div className="flex w-full gap-3 sm:w-auto">
          <Select
            aria-label="Kind"
            value={kind}
            onChange={(e) => setFilter('kind', e.target.value)}
            className="sm:w-44"
          >
            <option value="">All kinds</option>
            {kinds.map((k) => (
              <option key={k} value={k}>
                {kindLabel(k)}
              </option>
            ))}
          </Select>
          <Select
            aria-label="State"
            value={state?.param ?? ''}
            onChange={(e) => setFilter('state', e.target.value)}
            className="sm:w-44"
          >
            <option value="">All states</option>
            {states.map((s) => (
              <option key={s.param} value={s.param}>
                {s.label} ({countFor(counts, kind, s.state)})
              </option>
            ))}
          </Select>
        </div>
      </div>

      {isPending ? (
        <div className="flex justify-center py-16">
          <LoadingSpinner />
        </div>
      ) : error ? (
        <QueryError title="Could not load jobs" error={error} className="mt-8" />
      ) : jobs.length === 0 ? (
        <p className="py-10 text-center text-sm/6 text-zinc-500 dark:text-zinc-400">
          {kind || state ? 'No matching jobs' : 'No jobs'}
        </p>
      ) : (
        <Table className="mt-8 [--gutter:--spacing(6)] lg:[--gutter:--spacing(10)]">
          <TableHead>
            <TableRow>
              <TableHeader>Job</TableHeader>
              <TableHeader>State</TableHeader>
              <TableHeader>Priority</TableHeader>
              <TableHeader>Attempts</TableHeader>
              <TableHeader>Started by</TableHeader>
              <TableHeader>Created</TableHeader>
              <TableHeader className="text-right">Time</TableHeader>
              <TableHeader className="relative w-0">
                <span className="sr-only">Actions</span>
              </TableHeader>
            </TableRow>
          </TableHead>
          <TableBody>
            {jobs.map((j) => {
              const created = date(j.createdAt)
              const lastError = j.errors.at(-1)?.message
              return (
                <TableRow key={String(j.id)}>
                  <TableCell className="max-w-sm">
                    <div className="flex items-baseline gap-2">
                      <span className="font-medium text-zinc-950 dark:text-white">
                        {kindLabel(j.kind)}
                      </span>
                      <span className="text-zinc-500 tabular-nums dark:text-zinc-400">
                        #{String(j.id)}
                      </span>
                    </div>
                    <div className="truncate text-zinc-500 dark:text-zinc-400">
                      {j.target || '—'}
                    </div>
                    {lastError && j.state !== JobState.COMPLETED && (
                      <div
                        className="truncate text-xs/5 text-red-700 dark:text-red-400"
                        title={lastError}
                      >
                        {lastError}
                      </div>
                    )}
                  </TableCell>
                  <TableCell>
                    <StateBadge job={j} />
                    {j.gpu?.workerHostname && (
                      <div className="mt-1 font-mono text-xs text-zinc-500 dark:text-zinc-400">
                        {j.gpu.workerHostname}
                      </div>
                    )}
                  </TableCell>
                  <TableCell>
                    {notStarted(j) ? (
                      <Select
                        aria-label={`Priority of job ${j.id}`}
                        value={j.priority}
                        disabled={prioritize.isPending}
                        onChange={(e) =>
                          prioritize.mutate({ id: j.id, priority: Number(e.target.value) })
                        }
                        className="w-20"
                      >
                        {[1, 2, 3, 4].map((p) => (
                          <option key={p} value={p}>
                            {p}
                          </option>
                        ))}
                      </Select>
                    ) : (
                      <span className="text-zinc-500 tabular-nums dark:text-zinc-400">
                        {j.priority}
                      </span>
                    )}
                  </TableCell>
                  <TableCell className="text-zinc-500 tabular-nums dark:text-zinc-400">
                    {j.attempt} / {j.maxAttempts}
                  </TableCell>
                  <TableCell>
                    {j.startedBy ? (
                      <span className="flex items-center gap-2">
                        <Avatar seed={j.startedBy.id} className="size-6" />
                        <span className="truncate">{j.startedBy.name}</span>
                      </span>
                    ) : (
                      <span className="text-zinc-500 dark:text-zinc-400">—</span>
                    )}
                  </TableCell>
                  <TableCell
                    className="whitespace-nowrap text-zinc-500 dark:text-zinc-400"
                    title={created ? fullDate.format(created) : undefined}
                  >
                    {created ? formatRelative(created) : ''}
                  </TableCell>
                  <TableCell className="text-right text-zinc-500 tabular-nums dark:text-zinc-400">
                    {runTime(j, dataUpdatedAt) ?? '—'}
                  </TableCell>
                  <TableCell>
                    <Dropdown>
                      <DropdownButton plain aria-label={`Options for job ${j.id}`}>
                        <EllipsisHorizontalIcon data-slot="icon" />
                      </DropdownButton>
                      <DropdownMenu anchor="bottom end">
                        <DropdownItem onClick={() => setViewing(j)}>
                          <InformationCircleIcon data-slot="icon" />
                          <DropdownLabel>Details</DropdownLabel>
                        </DropdownItem>
                        {canRetry(j) && (
                          <DropdownItem onClick={() => retry.mutate({ id: j.id })}>
                            <ArrowPathIcon data-slot="icon" />
                            <DropdownLabel>
                              {j.state === JobState.RETRYABLE ? 'Retry now' : 'Retry'}
                            </DropdownLabel>
                          </DropdownItem>
                        )}
                        {canCancel(j) && (
                          <DropdownItem onClick={() => setCancelling(j)}>
                            <XMarkIcon data-slot="icon" />
                            <DropdownLabel>Cancel</DropdownLabel>
                          </DropdownItem>
                        )}
                      </DropdownMenu>
                    </Dropdown>
                  </TableCell>
                </TableRow>
              )
            })}
          </TableBody>
        </Table>
      )}
      {hasNextPage && (
        <div className="mt-6 flex justify-center">
          <Button outline disabled={isFetchingNextPage} onClick={() => void fetchNextPage()}>
            Load more
          </Button>
        </div>
      )}

      <JobDialog job={viewing} onClose={() => setViewing(null)} />
      <ConfirmAlert
        open={cancelling !== null}
        onClose={() => {
          setCancelling(null)
          cancel.reset()
        }}
        onConfirm={() => cancelling && cancel.mutate({ id: cancelling.id })}
        busy={cancel.isPending}
        error={cancel.error}
        title={`Cancel ${cancelling ? kindLabel(cancelling.kind).toLowerCase() : ''} job #${cancelling?.id ?? ''}?`}
        description={cancelling?.target ?? ''}
        confirmLabel="Cancel job"
      />
    </PageContentBlock>
  )
}
