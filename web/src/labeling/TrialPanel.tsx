import { useMutation, useQuery } from '@connectrpc/connect-query'
import { XMarkIcon } from '@heroicons/react/16/solid'
import { clsx } from 'clsx'
import { useMemo, useState } from 'react'
import { Meter } from '@/components/Meter'
import { SegmentedTabs } from '@/components/SegmentedTabs'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { Select } from '@/components/ui/Select'
import { Switch } from '@/components/ui/Switch'
import type { LabelType } from '@/gen/krill/v1/label_pb'
import {
  ModelTrialStatus,
  PipelineService,
  TrainingStatus,
  type TrainingRun,
} from '@/gen/krill/v1/pipeline_pb'
import { Permission } from '@/gen/krill/v1/user_pb'
import { percent, runName, trialActive, trialStatus } from '@/admin/pipeline'
import { useCan } from '@/lib/auth'
import { flash } from '@/lib/flash'
import { invalidateService } from '@/lib/queryClient'
import { precision, recall, type TrialReview } from './trialMatch'
import { MAX_THRESHOLD, MIN_THRESHOLD, useTrialStore, type TrialMode } from './useTrialStore'

const POLL_MS = 1500

const modes = [
  { key: 'boxes', label: 'Boxes' },
  { key: 'compare', label: 'Compare' },
] as const

function runLabel(r: TrainingRun) {
  const split = r.test?.all ? 'test' : 'val'
  const s = r.test?.all ?? r.val?.all
  return [runName(r.id), r.pinned && 'pinned', s && `${percent(s.map5095)} ${split}`]
    .filter(Boolean)
    .join(' · ')
}

function Stats({ review, types }: { review: TrialReview; types: LabelType[] }) {
  const byId = new Map(types.map((t) => [t.id, t]))
  const rows = [
    { key: 'all', name: 'All', s: review.all },
    ...review.classes.map((c) => ({
      key: String(c.labelTypeId),
      name: byId.get(c.labelTypeId)?.name ?? 'unknown',
      s: c,
    })),
  ]
  return (
    <table className="w-full text-xs/5 tabular-nums">
      <thead className="text-zinc-500 dark:text-zinc-400">
        <tr>
          <th className="text-left font-normal">{review.labeledFrames} frames</th>
          <th className="text-right font-normal">Match</th>
          <th className="text-right font-normal">Miss</th>
          <th className="text-right font-normal">Extra</th>
          <th className="text-right font-normal">P</th>
          <th className="text-right font-normal">R</th>
        </tr>
      </thead>
      <tbody>
        {rows.map(({ key, name, s }, i) => (
          <tr key={key} className={i === 0 ? 'font-medium' : undefined}>
            <td className="max-w-20 truncate font-mono">{name}</td>
            <td className="text-right">{s.matched}</td>
            <td className="text-right">{s.missed}</td>
            <td className="text-right">{s.extra}</td>
            <td className="text-right">{percent(precision(s))}</td>
            <td className="text-right">{percent(recall(s))}</td>
          </tr>
        ))}
      </tbody>
    </table>
  )
}

function Controls({ review, types }: { review: TrialReview | undefined; types: LabelType[] }) {
  const threshold = useTrialStore((s) => s.threshold)
  const mode = useTrialStore((s) => s.mode)
  const visible = useTrialStore((s) => s.visible)
  const setThreshold = useTrialStore((s) => s.setThreshold)
  const setMode = useTrialStore((s) => s.setMode)
  const toggleVisible = useTrialStore((s) => s.toggleVisible)
  return (
    <div className="space-y-3 border-t border-zinc-950/5 px-3 py-3 dark:border-white/5">
      <label className="block">
        <span className="flex justify-between text-xs/6 text-zinc-500 dark:text-zinc-400">
          Confidence
          <span className="text-zinc-950 tabular-nums dark:text-white">
            {Math.round(threshold * 100)}%
          </span>
        </span>
        <input
          type="range"
          min={MIN_THRESHOLD}
          max={MAX_THRESHOLD}
          step={0.05}
          value={threshold}
          onChange={(e) => setThreshold(Math.round(e.target.valueAsNumber * 100) / 100)}
          className="w-full accent-sky-500"
        />
      </label>
      <div className="flex items-center justify-between gap-2">
        <SegmentedTabs options={modes} value={mode} onChange={(k) => setMode(k as TrialMode)} />
        <Switch
          aria-label="Show model boxes"
          title="Show model boxes (Shift+B)"
          checked={visible}
          onChange={toggleVisible}
        />
      </div>
      {review && review.labeledFrames > 0 && <Stats review={review} types={types} />}
    </div>
  )
}

export function TrialPanel({
  clipId,
  types,
  review,
  selectedId,
  onSelect,
}: {
  clipId: bigint
  types: LabelType[]
  review: TrialReview | undefined
  selectedId: bigint | undefined
  onSelect: (id: bigint | undefined) => void
}) {
  const canReview = useCan(Permission.REVIEW_LABELS)
  const loaded = useTrialStore((s) => s.trial)
  const trials = useQuery(
    PipelineService.method.listModelTrials,
    { clipId },
    { refetchInterval: (q) => (q.state.data?.trials.some(trialActive) ? POLL_MS : false) },
  )
  const runs = useQuery(PipelineService.method.listTrainingRuns, {}, { enabled: canReview })
  const options = useMemo(
    () =>
      (runs.data?.runs ?? [])
        .filter((r) => r.status === TrainingStatus.SUCCEEDED)
        .sort((a, b) => Number(b.pinned) - Number(a.pinned)),
    [runs.data],
  )
  const [runId, setRunId] = useState(0n)
  const chosen = runId || options[0]?.id || 0n

  const start = useMutation(PipelineService.method.startModelTrial, {
    onSuccess: async (res) => {
      await invalidateService(PipelineService)
      if (res.trial) onSelect(res.trial.id)
    },
    onError: (err) => flash.error('Could not start trial', err),
  })
  const cancel = useMutation(PipelineService.method.cancelModelTrial, {
    onSuccess: () => invalidateService(PipelineService),
    onError: (err) => flash.error('Could not cancel trial', err),
  })

  const list = trials.data?.trials ?? []

  return (
    <section className="max-h-[60%] shrink-0 overflow-y-auto border-b border-zinc-950/10 dark:border-white/10">
      <div className="px-3 pt-3 pb-2">
        <h2 className="text-xs/6 font-medium text-zinc-500 dark:text-zinc-400">Model trials</h2>
      </div>
      {list.length > 0 && (
        <ul className="pb-2">
          {list.map((t) => {
            const selected = t.id === selectedId
            const active = trialActive(t)
            const meta = trialStatus[t.status]
            return (
              <li
                key={String(t.id)}
                className={clsx(
                  'flex items-center gap-1 pr-2',
                  selected && 'bg-zinc-950/[2.5%] dark:bg-white/5',
                )}
              >
                <button
                  type="button"
                  onClick={() => onSelect(selected ? undefined : t.id)}
                  title={t.error || undefined}
                  className="min-w-0 flex-1 px-3 py-1.5 text-left text-sm/5 hover:bg-zinc-950/5 dark:hover:bg-white/5"
                >
                  <span className="flex items-center gap-2">
                    <span className="flex-1 truncate">{runName(t.trainingRunId)}</span>
                    {t.status !== ModelTrialStatus.SUCCEEDED && (
                      <Badge color={meta.color}>{meta.label}</Badge>
                    )}
                    {active && (
                      <span className="text-xs text-zinc-500 tabular-nums">
                        {t.framesDone}/{t.framesTotal}
                      </span>
                    )}
                  </span>
                  {active && t.framesTotal > 0 && (
                    <Meter
                      label="Frames done"
                      value={t.framesDone / t.framesTotal}
                      className="mt-1.5"
                    />
                  )}
                </button>
                {active && canReview && (
                  <Button
                    plain
                    title="Cancel"
                    disabled={cancel.isPending}
                    onClick={() => cancel.mutate({ id: t.id })}
                  >
                    <XMarkIcon data-slot="icon" />
                  </Button>
                )}
              </li>
            )
          })}
        </ul>
      )}
      {canReview && options.length > 0 && (
        <div className="flex gap-2 px-3 pb-3">
          <Select
            aria-label="Run"
            value={String(chosen)}
            onChange={(e) => setRunId(BigInt(e.target.value))}
          >
            {options.map((r) => (
              <option key={String(r.id)} value={String(r.id)}>
                {runLabel(r)}
              </option>
            ))}
          </Select>
          <Button
            outline
            disabled={!chosen || start.isPending}
            onClick={() => start.mutate({ trainingRunId: chosen, clipId })}
          >
            Run
          </Button>
        </div>
      )}
      {loaded && loaded.id === selectedId && <Controls review={review} types={types} />}
    </section>
  )
}
