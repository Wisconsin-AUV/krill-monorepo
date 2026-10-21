import { useMutation, useQuery } from '@connectrpc/connect-query'
import { BeakerIcon } from '@heroicons/react/20/solid'
import { useState } from 'react'
import { useNavigate } from 'react-router'
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
import { PipelineService } from '@/gen/krill/v1/pipeline_pb'
import { Permission } from '@/gen/krill/v1/user_pb'
import { SplitAssignment, VideoService, VideoStatus } from '@/gen/krill/v1/video_pb'
import { useCan } from '@/lib/auth'
import { formatNumber } from '@/lib/format'
import { invalidateService } from '@/lib/queryClient'
import { percent, trialActive, trialStatus } from './pipeline'

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
      if (res.trial) navigate(`/clips/${res.trial.clipId}?trial=${res.trial.id}`)
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
        start.mutate({ trainingRunId: runId, clipId })
      }}
    >
      <DialogTitle>Try on a clip</DialogTitle>
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
                <option value="0">Choose…</option>
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
        <Button type="submit" color="sky" disabled={clipId === 0n || start.isPending}>
          Run
        </Button>
      </DialogActions>
    </form>
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

  return (
    <section className="mt-12">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <Subheading>Trials</Subheading>
        {canReview && (
          <Button outline onClick={() => setOpen(true)}>
            <BeakerIcon data-slot="icon" />
            Try on a clip
          </Button>
        )}
      </div>
      {error && <QueryError title="Could not load trials" error={error} className="mt-4" />}
      {trials.length > 0 && (
        <Table dense className="mt-4 [--gutter:--spacing(6)] lg:[--gutter:--spacing(10)]">
          <TableHead>
            <TableRow>
              <TableHeader>Video</TableHeader>
              <TableHeader className="text-right">Clip</TableHeader>
              <TableHeader>Status</TableHeader>
              <TableHeader className="text-right">Frames</TableHeader>
              <TableHeader className="text-right">Precision</TableHeader>
              <TableHeader className="text-right">Recall</TableHeader>
              <TableHeader className="text-right">Missed</TableHeader>
              <TableHeader className="text-right">Extra</TableHeader>
            </TableRow>
          </TableHead>
          <TableBody>
            {trials.map((t) => {
              const meta = trialStatus[t.status]
              const all = t.summary?.all
              return (
                <TableRow
                  key={String(t.id)}
                  href={`/clips/${t.clipId}?trial=${t.id}`}
                  title={`${t.videoName} · Clip ${t.clipIndex + 1}`}
                >
                  <TableCell className="font-medium">{t.videoName}</TableCell>
                  <TableCell className="text-right tabular-nums">{t.clipIndex + 1}</TableCell>
                  <TableCell>
                    <Badge color={meta.color} title={t.error || undefined}>
                      {meta.label}
                    </Badge>
                  </TableCell>
                  <TableCell className="text-right tabular-nums">
                    {formatNumber(t.framesDone)}/{formatNumber(t.framesTotal)}
                  </TableCell>
                  <TableCell className="text-right tabular-nums">
                    {percent(all?.precision)}
                  </TableCell>
                  <TableCell className="text-right tabular-nums">{percent(all?.recall)}</TableCell>
                  <TableCell className="text-right tabular-nums">
                    {all ? formatNumber(all.missed) : '—'}
                  </TableCell>
                  <TableCell className="text-right tabular-nums">
                    {all ? formatNumber(all.extra) : '—'}
                  </TableCell>
                </TableRow>
              )
            })}
          </TableBody>
        </Table>
      )}
      <Dialog open={open} onClose={() => setOpen(false)}>
        {open && <TrialForm runId={runId} onClose={() => setOpen(false)} />}
      </Dialog>
    </section>
  )
}
