import { useMutation, useQuery } from '@connectrpc/connect-query'
import { ChevronRightIcon } from '@heroicons/react/16/solid'
import { BeakerIcon } from '@heroicons/react/20/solid'
import { clsx } from 'clsx'
import { useEffect, useMemo, useRef, useState } from 'react'
import { Link, useNavigate } from 'react-router'
import { Thumbnail } from '@/components/Thumbnail'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { FormError, QueryError } from '@/components/ui/Callout'
import { Dialog, DialogActions, DialogBody, DialogTitle } from '@/components/ui/Dialog'
import { Field, FieldGroup, Label } from '@/components/ui/Fieldset'
import { Subheading } from '@/components/ui/Heading'
import { Select } from '@/components/ui/Select'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/Table'
import { TextButton } from '@/components/ui/Text'
import { LabelService } from '@/gen/krill/v1/label_pb'
import { ModelTrialStatus, PipelineService, type ModelTrialClass } from '@/gen/krill/v1/pipeline_pb'
import { Permission } from '@/gen/krill/v1/user_pb'
import { SplitAssignment, VideoService, VideoStatus } from '@/gen/krill/v1/video_pb'
import { useCan } from '@/lib/auth'
import { flash } from '@/lib/flash'
import { formatNumber } from '@/lib/format'
import { invalidateService } from '@/lib/queryClient'
import { percent, trialActive, trialStatus } from './pipeline'
import { groupTrials, scoreTrials, type TrialScore, type VideoTrials } from './trials'

const POLL = 5000

function TrialForm({ runId, onClose }: { runId: bigint; onClose: () => void }) {
  const navigate = useNavigate()
  const [videoId, setVideoId] = useState(0n)
  const [clipId, setClipId] = useState(0n)
  const videos = useQuery(VideoService.method.listVideos, {})
  const video = useQuery(VideoService.method.getVideo, { id: videoId }, { enabled: videoId > 0n })
  const ready = (videos.data?.videos ?? []).filter((v) => v.status === VideoStatus.READY)
  const test = ready.filter((v) => v.split === SplitAssignment.TEST)
  const rest = ready.filter((v) => v.split !== SplitAssignment.TEST)

  const start = useMutation(PipelineService.method.startModelTrial, {
    onSuccess: async (res) => {
      await invalidateService(PipelineService)
      onClose()
      if (res.trial && clipId > 0n) navigate(`/clips/${res.trial.clipId}?trial=${res.trial.id}`)
    },
  })

  const option = (v: (typeof ready)[number]) => (
    <option key={String(v.id)} value={String(v.id)}>
      {v.name}
    </option>
  )

  return (
    <form
      onSubmit={(e) => {
        e.preventDefault()
        start.mutate(
          clipId > 0n ? { trainingRunId: runId, clipId } : { trainingRunId: runId, videoId },
        )
      }}
    >
      <DialogTitle>Try on a video</DialogTitle>
      <DialogBody>
        <FieldGroup>
          <div className="grid gap-6 sm:grid-cols-2">
            <Field>
              <Label>Video</Label>
              <Select
                value={String(videoId)}
                onChange={(e) => {
                  setVideoId(BigInt(e.target.value))
                  setClipId(0n)
                }}
              >
                <option value="0">Choose…</option>
                {test.length > 0 ? (
                  <>
                    <optgroup label="Held-out test">{test.map(option)}</optgroup>
                    <optgroup label="Other">{rest.map(option)}</optgroup>
                  </>
                ) : (
                  rest.map(option)
                )}
              </Select>
            </Field>
            <Field disabled={!video.data}>
              <Label>Clip</Label>
              <Select value={String(clipId)} onChange={(e) => setClipId(BigInt(e.target.value))}>
                <option value="0">{videoId > 0n ? 'Whole video' : 'Choose…'}</option>
                {(video.data?.clips ?? []).map((c) => (
                  <option key={String(c.id)} value={String(c.id)}>
                    Clip {c.index + 1}
                  </option>
                ))}
              </Select>
            </Field>
          </div>
          <FormError error={start.error ?? videos.error} />
        </FieldGroup>
      </DialogBody>
      <DialogActions>
        <Button plain onClick={onClose}>
          Cancel
        </Button>
        <Button type="submit" color="sky" disabled={videoId === 0n || start.isPending}>
          Run
        </Button>
      </DialogActions>
    </form>
  )
}

function Score({ score }: { score: TrialScore | undefined }) {
  return (
    <>
      <TableCell className="text-right tabular-nums">{percent(score?.precision)}</TableCell>
      <TableCell className="text-right tabular-nums">{percent(score?.recall)}</TableCell>
      <TableCell className="text-right tabular-nums">
        {score ? formatNumber(score.missed) : '—'}
      </TableCell>
      <TableCell className="text-right tabular-nums">
        {score ? formatNumber(score.extra) : '—'}
      </TableCell>
    </>
  )
}

function VideoRows({ group, canReview }: { group: VideoTrials; canReview: boolean }) {
  const [open, setOpen] = useState(false)
  const [cancelling, setCancelling] = useState(false)
  const cancel = useMutation(PipelineService.method.cancelModelTrial)
  const active = group.trials.filter(trialActive)

  async function cancelAll() {
    setCancelling(true)
    try {
      await Promise.all(active.map((t) => cancel.mutateAsync({ id: t.id })))
    } catch (err) {
      flash.error('Could not cancel trials', err)
    } finally {
      await invalidateService(PipelineService)
      setCancelling(false)
    }
  }

  return (
    <>
      <TableRow
        onClick={() => setOpen((o) => !o)}
        className="cursor-pointer hover:bg-zinc-950/[2.5%] dark:hover:bg-white/[2.5%]"
      >
        <TableCell className="font-medium">
          <button
            type="button"
            aria-expanded={open}
            className="flex items-center gap-2 rounded-sm focus:outline-none focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-500"
          >
            <ChevronRightIcon
              className={clsx(
                'size-4 fill-zinc-400 transition-transform duration-150 dark:fill-zinc-500',
                open && 'rotate-90',
              )}
            />
            {group.videoName}
          </button>
        </TableCell>
        <TableCell>
          <span className="flex items-center gap-2">
            <span className="tabular-nums">
              {formatNumber(group.done)} of {formatNumber(group.trials.length)} done
            </span>
            {group.running > 0 && <Badge color="sky">{formatNumber(group.running)} running</Badge>}
            {group.queued > 0 && <Badge color="zinc">{formatNumber(group.queued)} queued</Badge>}
            {group.failed > 0 && <Badge color="red">{formatNumber(group.failed)} failed</Badge>}
          </span>
        </TableCell>
        <TableCell className="text-right tabular-nums">
          {formatNumber(group.framesDone)}/{formatNumber(group.framesTotal)}
        </TableCell>
        <Score score={group.score} />
        <TableCell className="text-right">
          {canReview && active.length > 0 && (
            <TextButton
              onClick={(e) => {
                e.stopPropagation()
                void cancelAll()
              }}
              disabled={cancelling}
            >
              Cancel
            </TextButton>
          )}
        </TableCell>
      </TableRow>
      {open &&
        group.trials.map((t) => {
          const meta = trialStatus[t.status]
          return (
            <TableRow
              key={String(t.id)}
              href={`/clips/${t.clipId}?trial=${t.id}`}
              title={`${t.videoName} · Clip ${t.clipIndex + 1}`}
            >
              <TableCell>
                <span className="pl-6 tabular-nums">Clip {t.clipIndex + 1}</span>
              </TableCell>
              <TableCell>
                <Badge color={meta.color} title={t.error || undefined}>
                  {meta.label}
                </Badge>
              </TableCell>
              <TableCell className="text-right tabular-nums">
                {formatNumber(t.framesDone)}/{formatNumber(t.framesTotal)}
              </TableCell>
              <Score score={scoreTrials([t])} />
              <TableCell />
            </TableRow>
          )
        })}
    </>
  )
}

function breakdown(c: ModelTrialClass) {
  const wrong = c.matched - c.correct
  return [
    c.missed > 0 && `${formatNumber(c.missed)} missed`,
    c.extra > 0 && `${formatNumber(c.extra)} extra`,
    wrong > 0 && `${formatNumber(wrong)} wrong`,
  ]
    .filter(Boolean)
    .join(', ')
}

function TrialErrors({
  runId,
  videos,
  active,
}: {
  runId: bigint
  videos: VideoTrials[]
  active: boolean
}) {
  const [videoId, setVideoId] = useState(0n)
  const [labelTypeId, setLabelTypeId] = useState(0n)
  const types = useQuery(LabelService.method.listLabelTypes, {})
  const { data, error, refetch } = useQuery(
    PipelineService.method.listModelTrialErrors,
    { trainingRunId: runId, videoId, labelTypeId, limit: 60 },
    { refetchInterval: active ? POLL : false },
  )

  const wasActive = useRef(active)
  useEffect(() => {
    if (wasActive.current && !active) void refetch()
    wasActive.current = active
  }, [active, refetch])

  return (
    <section className="mt-12">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <Subheading>Errors</Subheading>
        <div className="flex w-full gap-3 sm:w-auto">
          <Select
            aria-label="Video"
            value={String(videoId)}
            onChange={(e) => setVideoId(BigInt(e.target.value))}
            className="sm:w-48"
          >
            <option value="0">All videos</option>
            {videos.map((v) => (
              <option key={String(v.videoId)} value={String(v.videoId)}>
                {v.videoName}
              </option>
            ))}
          </Select>
          <Select
            aria-label="Label type"
            value={String(labelTypeId)}
            onChange={(e) => setLabelTypeId(BigInt(e.target.value))}
            className="sm:w-48"
          >
            <option value="0">All types</option>
            {(types.data?.labelTypes ?? []).map((t) => (
              <option key={String(t.id)} value={String(t.id)}>
                {t.name}
              </option>
            ))}
          </Select>
        </div>
      </div>
      {error && <QueryError title="Could not load errors" error={error} className="mt-4" />}
      {data && data.totalFrames > 0 && (
        <p className="mt-4 text-sm/6 text-zinc-500 tabular-nums dark:text-zinc-400">
          {formatNumber(data.errorFrames)} of {formatNumber(data.totalFrames)} labeled frames{' '}
          {data.errorFrames === 1 ? 'has' : 'have'} an error
        </p>
      )}
      {data && data.frames.length > 0 && (
        <ul className="mt-4 grid gap-6 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
          {data.frames.map((f) => {
            const href = `/clips/${f.clipId}?trial=${f.trialId}&frame=${f.frameIndex}&mode=compare`
            const caption = `${f.videoName} · Clip ${f.clipIndex + 1} · Frame ${f.frameIndex + 1}`
            return (
              <li key={`${f.trialId}-${f.frameId}`}>
                <Link to={href} className="block">
                  <Thumbnail src={f.thumbnailUrl} alt="" />
                </Link>
                <div className="mt-2 min-w-0 text-sm/5">
                  <Link
                    to={href}
                    className="block truncate font-medium text-zinc-950 hover:underline dark:text-white"
                  >
                    {caption}
                  </Link>
                  <div className="mt-1 flex flex-wrap gap-1.5">
                    {f.missed > 0 && <Badge color="red">{formatNumber(f.missed)} missed</Badge>}
                    {f.extra > 0 && <Badge color="orange">{formatNumber(f.extra)} extra</Badge>}
                    {f.wrong > 0 && <Badge color="yellow">{formatNumber(f.wrong)} wrong</Badge>}
                  </div>
                  {f.classes.length > 0 && (
                    <div className="mt-1 text-xs/5 text-zinc-500 tabular-nums dark:text-zinc-400">
                      {f.classes.map((c) => `${c.name}: ${breakdown(c)}`).join(' · ')}
                    </div>
                  )}
                </div>
              </li>
            )
          })}
        </ul>
      )}
    </section>
  )
}

export function RunTrials({ runId }: { runId: bigint }) {
  const canReview = useCan(Permission.REVIEW_LABELS)
  const [open, setOpen] = useState(false)
  const { data, error } = useQuery(
    PipelineService.method.listModelTrials,
    { trainingRunId: runId },
    { refetchInterval: (q) => (q.state.data?.trials.some(trialActive) ? POLL : false) },
  )
  const trials = data?.trials ?? []
  const groups = useMemo(() => groupTrials(data?.trials ?? []), [data])

  return (
    <>
      <section className="mt-12">
        <div className="flex flex-wrap items-center justify-between gap-4">
          <Subheading>Trials</Subheading>
          {canReview && (
            <Button outline onClick={() => setOpen(true)}>
              <BeakerIcon data-slot="icon" />
              Try on a video
            </Button>
          )}
        </div>
        {error && <QueryError title="Could not load trials" error={error} className="mt-4" />}
        {groups.length > 0 && (
          <Table dense className="mt-4 [--gutter:--spacing(6)] lg:[--gutter:--spacing(10)]">
            <TableHead>
              <TableRow>
                <TableHeader>Video</TableHeader>
                <TableHeader>Status</TableHeader>
                <TableHeader className="text-right">Frames</TableHeader>
                <TableHeader className="text-right">Precision</TableHeader>
                <TableHeader className="text-right">Recall</TableHeader>
                <TableHeader className="text-right">Missed</TableHeader>
                <TableHeader className="text-right">Extra</TableHeader>
                <TableHeader>
                  <span className="sr-only">Actions</span>
                </TableHeader>
              </TableRow>
            </TableHead>
            <TableBody>
              {groups.map((g) => (
                <VideoRows key={String(g.videoId)} group={g} canReview={canReview} />
              ))}
            </TableBody>
          </Table>
        )}
        <Dialog open={open} onClose={() => setOpen(false)}>
          {open && <TrialForm runId={runId} onClose={() => setOpen(false)} />}
        </Dialog>
      </section>
      {trials.some((t) => t.status === ModelTrialStatus.SUCCEEDED) && (
        <TrialErrors runId={runId} videos={groups} active={trials.some(trialActive)} />
      )}
    </>
  )
}
