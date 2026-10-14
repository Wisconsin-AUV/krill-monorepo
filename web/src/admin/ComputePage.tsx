import { timestampDate } from '@bufbuild/protobuf/wkt'
import { useMutation, useQuery } from '@connectrpc/connect-query'
import { CircleStackIcon, CpuChipIcon, ServerIcon } from '@heroicons/react/20/solid'
import { EmptyState } from '@/components/EmptyState'
import { Meter } from '@/components/Meter'
import { Badge, BadgeButton, type BadgeColor } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { QueryError } from '@/components/ui/Callout'
import { Heading } from '@/components/ui/Heading'
import LoadingSpinner from '@/components/ui/LoadingSpinner'
import PageContentBlock from '@/components/ui/PageContentBlock'
import {
  AdminService,
  type ComputeNode,
  type ComputeSample,
  type GpuQueue,
  type ObjectStorage,
  NodeRole,
  type RunningTask,
} from '@/gen/krill/v1/admin_pb'
import { ComputeService } from '@/gen/krill/v1/compute_pb'
import type { GpuStats } from '@/gen/krill/v1/system_pb'
import { ModelLoadState, WorkerAction } from '@/gen/krill/v1/worker_pb'
import { flash } from '@/lib/flash'
import { formatBytes, formatDuration, formatRelative, plural } from '@/lib/format'
import { invalidateService } from '@/lib/queryClient'
import { Sparkline } from './Sparkline'

const INTERVAL = 5

const percent = (v: number) => `${Math.round(v)}%`
const bytes = (v: number) => formatBytes(v)

function Metric({
  label,
  value,
  detail,
  children,
}: {
  label: string
  value: React.ReactNode
  detail?: React.ReactNode
  children?: React.ReactNode
}) {
  return (
    <div className="min-w-0">
      <p className="text-sm/6 text-zinc-500 dark:text-zinc-400">{label}</p>
      <p className="mt-1 flex items-baseline gap-2">
        <span className="text-2xl/8 font-semibold text-zinc-950 tabular-nums dark:text-white">
          {value}
        </span>
        {detail && (
          <span className="truncate text-sm/6 text-zinc-500 tabular-nums dark:text-zinc-400">
            {detail}
          </span>
        )}
      </p>
      {children && <div className="mt-3">{children}</div>}
    </div>
  )
}

const taskNames: Record<string, string> = {
  track: 'Tracking',
  exemplar: 'Finding similar objects',
  embed: 'Embedding',
}

function Task({ task, now }: { task: RunningTask; now: number }) {
  const started = task.startedAt ? timestampDate(task.startedAt) : null
  return (
    <p className="flex items-center gap-2 text-sm/6 text-zinc-700 dark:text-zinc-300">
      <span className="relative flex size-2">
        <span className="absolute inline-flex size-full animate-ping rounded-full bg-sky-400 opacity-75 motion-reduce:hidden" />
        <span className="relative inline-flex size-2 rounded-full bg-sky-500" />
      </span>
      {taskNames[task.kind] ?? task.kind} {plural(task.frames, 'frame')}
      {started && (
        <span className="text-zinc-500 tabular-nums dark:text-zinc-400">
          for {formatDuration(now - started.getTime())}
        </span>
      )}
    </p>
  )
}

function Gpu({ gpu, history }: { gpu: GpuStats; history: ComputeSample[] }) {
  const samples = history.map((s) => s.gpus[gpu.index])
  const extras = [
    gpu.temperatureCelsius !== undefined && `${Math.round(gpu.temperatureCelsius)} °C`,
    gpu.powerWatts !== undefined && `${Math.round(gpu.powerWatts)} W`,
  ].filter(Boolean)
  return (
    <div className="border-t border-zinc-950/5 pt-6 dark:border-white/5">
      <p className="flex flex-wrap items-baseline gap-x-3 text-sm/6">
        <span className="font-medium text-zinc-950 dark:text-white">{gpu.name}</span>
        {extras.length > 0 && (
          <span className="text-zinc-500 tabular-nums dark:text-zinc-400">
            {extras.join(' · ')}
          </span>
        )}
      </p>
      <div className="mt-4 grid gap-8 sm:grid-cols-2">
        <Metric
          label="GPU"
          value={gpu.utilizationPercent !== undefined ? percent(gpu.utilizationPercent) : 'Unknown'}
        >
          {gpu.utilizationPercent !== undefined && (
            <Sparkline
              label="GPU utilization"
              values={samples.map((s) => s?.utilizationPercent)}
              max={100}
              format={percent}
              interval={INTERVAL}
            />
          )}
        </Metric>
        <Metric
          label="GPU memory"
          value={formatBytes(Number(gpu.memoryUsedBytes))}
          detail={`of ${formatBytes(Number(gpu.memoryTotalBytes))}`}
        >
          <Sparkline
            label="GPU memory"
            values={samples.map((s) => (s ? Number(s.memoryUsedBytes) : undefined))}
            max={Number(gpu.memoryTotalBytes)}
            format={bytes}
            interval={INTERVAL}
          />
        </Metric>
      </div>
    </div>
  )
}

const samModels: Record<string, string> = {
  'facebook/sam2.1-hiera-tiny': 'SAM 2.1 tiny',
  'facebook/sam2.1-hiera-small': 'SAM 2.1 small',
  'facebook/sam2.1-hiera-base-plus': 'SAM 2.1 base plus',
  'facebook/sam2.1-hiera-large': 'SAM 2.1 large',
  'facebook/sam3': 'SAM 3',
}
const samName = (id: string) => samModels[id] ?? id

const modelStates: Record<ModelLoadState, { label: string; color: BadgeColor }> = {
  [ModelLoadState.UNSPECIFIED]: { label: 'Unknown', color: 'zinc' },
  [ModelLoadState.UNLOADED]: { label: 'Unloaded', color: 'zinc' },
  [ModelLoadState.LOADING]: { label: 'Loading', color: 'amber' },
  [ModelLoadState.READY]: { label: 'Ready', color: 'green' },
  [ModelLoadState.ERROR]: { label: 'Failed to load', color: 'red' },
}

function WorkerModelRow({ node, wanted }: { node: ComputeNode; wanted: string }) {
  const sam = node.sam
  const control = useMutation(ComputeService.method.controlWorker, {
    onSuccess: () => invalidateService(AdminService),
    onError: (err) => flash.error('Could not reach the worker', err),
  })
  if (!sam) return null
  const state = modelStates[sam.state]
  const pending = sam.pendingAction !== WorkerAction.UNSPECIFIED || control.isPending
  const switching = !sam.pinned && wanted !== '' && node.model !== wanted
  const send = (action: WorkerAction) => control.mutate({ workerId: node.id, action })
  const loaded = sam.state === ModelLoadState.READY

  return (
    <div className="flex flex-wrap items-center gap-x-6 gap-y-3 border-b border-zinc-950/5 px-6 py-4 dark:border-white/5">
      <div className="min-w-0 flex-1">
        <p className="flex flex-wrap items-center gap-2 text-sm/6">
          <span className="font-medium text-zinc-950 dark:text-white">{samName(node.model)}</span>
          <Badge color={state.color}>{state.label}</Badge>
          {sam.pinned && <Badge title="Set by the worker's KRILL_SAM_MODEL">Pinned</Badge>}
          {switching && <Badge color="sky">Switching to {samName(wanted)}</Badge>}
          {sam.supportsExemplars && <Badge color="violet">Similar objects</Badge>}
        </p>
        <p className="text-sm/6 text-zinc-500 tabular-nums dark:text-zinc-400">
          {[
            loaded &&
              sam.memoryReservedBytes > 0n &&
              `${formatBytes(Number(sam.memoryAllocatedBytes))} allocated · ${formatBytes(Number(sam.memoryReservedBytes))} reserved`,
            node.tokenName && `Token ${node.tokenName}`,
            node.lastSeen && `heartbeat ${formatRelative(timestampDate(node.lastSeen))}`,
          ]
            .filter(Boolean)
            .join(' · ')}
        </p>
        {sam.error && (
          <p className="text-sm/6 break-words text-red-700 dark:text-red-400">{sam.error}</p>
        )}
        {sam.cached.length > 0 && (
          <p className="text-sm/6 text-zinc-500 dark:text-zinc-400">
            Downloaded: {sam.cached.map(samName).join(', ')}
          </p>
        )}
      </div>
      {node.online && (
        <div className="flex gap-2">
          <Button
            outline
            disabled={pending || sam.state === ModelLoadState.LOADING || (loaded && !switching)}
            onClick={() => send(WorkerAction.WARM_UP)}
          >
            {sam.pendingAction === WorkerAction.WARM_UP ? 'Warming up' : 'Warm up'}
          </Button>
          <Button
            outline
            disabled={
              pending ||
              sam.state === ModelLoadState.UNLOADED ||
              sam.state === ModelLoadState.LOADING
            }
            onClick={() => send(WorkerAction.UNLOAD)}
          >
            {sam.pendingAction === WorkerAction.UNLOAD ? 'Unloading' : 'Unload'}
          </Button>
        </div>
      )}
    </div>
  )
}

function NodeCard({ node, now, wanted }: { node: ComputeNode; now: number; wanted: string }) {
  const api = node.role === NodeRole.API
  const Icon = api ? ServerIcon : CpuChipIcon
  const host = node.host
  const started = node.startedAt && timestampDate(node.startedAt)
  const details = [node.accelerator, host?.platform].filter(Boolean)

  return (
    <section className="rounded-xl ring-1 ring-zinc-950/10 dark:ring-white/10">
      <header className="flex flex-wrap items-start gap-x-4 gap-y-2 border-b border-zinc-950/5 p-6 dark:border-white/5">
        <div className="flex size-10 shrink-0 items-center justify-center rounded-lg bg-zinc-950/5 dark:bg-white/5">
          <Icon className="size-5 fill-zinc-500 dark:fill-zinc-400" />
        </div>
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-2">
            <h2 className="text-base/7 font-semibold text-zinc-950 sm:text-sm/6 dark:text-white">
              {api ? 'API' : 'GPU worker'}
            </h2>
            <span className="font-mono text-sm/6 text-zinc-500 dark:text-zinc-400">
              {node.hostname}
            </span>
            {node.version && (
              <Badge>{node.version.startsWith('v') ? node.version : `v${node.version}`}</Badge>
            )}
            {!node.online && <Badge color="red">Offline</Badge>}
          </div>
          <p className="text-sm/6 text-zinc-500 dark:text-zinc-400">
            {details.join(' · ')}
            {details.length > 0 && started && ' · '}
            {started && `started ${formatRelative(started)}`}
          </p>
        </div>
        {node.task && <Task task={node.task} now={now} />}
      </header>
      {!api && <WorkerModelRow node={node} wanted={wanted} />}
      {!node.online ? (
        <p className="p-6 text-sm/6 text-zinc-500 dark:text-zinc-400">
          Last seen {node.lastSeen ? formatRelative(timestampDate(node.lastSeen)) : 'never'}
        </p>
      ) : (
        host && (
          <div className="space-y-6 p-6">
            <div className="grid gap-8 sm:grid-cols-2 lg:grid-cols-3">
              <Metric
                label="CPU"
                value={percent(host.cpuPercent)}
                detail={plural(host.cpuCount, 'core')}
              >
                <Sparkline
                  label="CPU"
                  values={node.history.map((s) => s.cpuPercent)}
                  max={100}
                  format={percent}
                  interval={INTERVAL}
                />
              </Metric>
              <Metric
                label="Memory"
                value={formatBytes(Number(host.memoryUsedBytes))}
                detail={`of ${formatBytes(Number(host.memoryTotalBytes))}`}
              >
                <Sparkline
                  label="Memory"
                  values={node.history.map((s) => Number(s.memoryUsedBytes))}
                  max={Number(host.memoryTotalBytes)}
                  format={bytes}
                  interval={INTERVAL}
                />
              </Metric>
              {host.diskTotalBytes > 0n && (
                <Metric
                  label="Disk"
                  value={formatBytes(Number(host.diskUsedBytes))}
                  detail={`of ${formatBytes(Number(host.diskTotalBytes))}`}
                >
                  <Meter
                    label="Disk used"
                    value={Number(host.diskUsedBytes) / Number(host.diskTotalBytes)}
                  />
                </Metric>
              )}
            </div>
            {node.gpus.map((g) => (
              <Gpu key={g.index} gpu={g} history={node.history} />
            ))}
          </div>
        )
      )}
    </section>
  )
}

function Storage({ storage }: { storage: ObjectStorage }) {
  const total = Number(storage.totalBytes)
  const used = Number(storage.usedBytes)
  return (
    <section className="flex flex-wrap items-center gap-x-6 gap-y-4 rounded-xl p-6 ring-1 ring-zinc-950/10 dark:ring-white/10">
      <div className="flex size-10 shrink-0 items-center justify-center rounded-lg bg-zinc-950/5 dark:bg-white/5">
        <CircleStackIcon className="size-5 fill-zinc-500 dark:fill-zinc-400" />
      </div>
      <div className="min-w-48 flex-1">
        <h2 className="text-base/7 font-semibold text-zinc-950 sm:text-sm/6 dark:text-white">
          Storage
        </h2>
        <p className="text-sm/6 text-zinc-500 dark:text-zinc-400">
          {storage.error
            ? 'Usage unknown. MinIO only reports it to its root user.'
            : 'Frames, embeddings, and exports'}
        </p>
      </div>
      {total > 0 && (
        <div className="w-full sm:w-80">
          <p className="flex items-baseline justify-between gap-2 text-sm/6 tabular-nums">
            <span>
              <span className="font-semibold text-zinc-950 dark:text-white">
                {formatBytes(used)}
              </span>
              <span className="text-zinc-500 dark:text-zinc-400"> of {formatBytes(total)}</span>
            </span>
            <span className="text-zinc-500 dark:text-zinc-400">
              {formatBytes(total - used)} free
            </span>
          </p>
          <Meter label="Storage used" value={used / total} className="mt-2" />
        </div>
      )}
    </section>
  )
}

function Queue({ queue }: { queue: GpuQueue }) {
  const waiting = queue.interactive + queue.background
  if (waiting === 0) return <BadgeButton to="/admin/jobs">Queue empty</BadgeButton>
  return (
    <div className="flex gap-2">
      {queue.interactive > 0 && (
        <BadgeButton color="sky" to="/admin/jobs?state=queued">
          {queue.interactive} waiting for tracking
        </BadgeButton>
      )}
      {queue.background > 0 && (
        <BadgeButton to="/admin/jobs?kind=embed&state=queued">
          {queue.background} embeddings queued
        </BadgeButton>
      )}
    </div>
  )
}

export function ComputePage() {
  const { data, isPending, error, dataUpdatedAt } = useQuery(
    AdminService.method.getCompute,
    {},
    { refetchInterval: INTERVAL * 1000 },
  )
  const workers = data?.nodes.filter((n) => n.role === NodeRole.WORKER) ?? []

  return (
    <PageContentBlock title="Compute · Admin · Krill">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <Heading>Compute</Heading>
        {data?.queue && <Queue queue={data.queue} />}
      </div>
      {isPending ? (
        <div className="flex justify-center py-16">
          <LoadingSpinner />
        </div>
      ) : error ? (
        <QueryError title="Could not load compute" error={error} className="mt-8" />
      ) : (
        <div className="mt-8 space-y-6">
          {data.storage && <Storage storage={data.storage} />}
          {data.nodes.map((n) => (
            <NodeCard key={n.id} node={n} now={dataUpdatedAt} wanted={data.samModel} />
          ))}
          {workers.length === 0 && (
            <EmptyState
              icon={CpuChipIcon}
              title="No GPU worker connected"
              description="Tracking and embeddings wait until one connects."
            />
          )}
        </div>
      )}
    </PageContentBlock>
  )
}
