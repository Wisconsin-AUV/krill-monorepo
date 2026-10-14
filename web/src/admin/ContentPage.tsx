import { timestampDate } from '@bufbuild/protobuf/wkt'
import { useMutation, useQuery } from '@connectrpc/connect-query'
import { ArrowPathIcon, CpuChipIcon, UserMinusIcon, UserPlusIcon } from '@heroicons/react/20/solid'
import { useState } from 'react'
import { Link } from 'react-router'
import { Meter } from '@/components/Meter'
import { SegmentedTabs } from '@/components/SegmentedTabs'
import { VideoStatusBadge } from '@/components/VideoStatusBadge'
import { Avatar } from '@/components/ui/Avatar'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { FormError, QueryError } from '@/components/ui/Callout'
import { Checkbox, CheckboxField } from '@/components/ui/Checkbox'
import { Dialog, DialogActions, DialogBody, DialogTitle } from '@/components/ui/Dialog'
import { Field, Label } from '@/components/ui/Fieldset'
import { Heading, Subheading } from '@/components/ui/Heading'
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
import { ClipService } from '@/gen/krill/v1/clip_pb'
import {
  ContentJobKind,
  ContentService,
  type ContentClip,
  type ContentJob,
  type ContentVideo,
} from '@/gen/krill/v1/content_pb'
import { QueueService } from '@/gen/krill/v1/queue_pb'
import { UserService } from '@/gen/krill/v1/user_pb'
import { VideoService, VideoStatus } from '@/gen/krill/v1/video_pb'
import { flash } from '@/lib/flash'
import { formatNumber, formatRelative, plural } from '@/lib/format'
import { invalidateService } from '@/lib/queryClient'
import { extractFpsOptions } from '@/lib/video'
import { useSelection } from './selection'

const tabs = [
  { key: 'videos', label: 'Videos' },
  { key: 'clips', label: 'Clips' },
] as const

const jobKinds: Record<ContentJobKind, string> = {
  [ContentJobKind.UNSPECIFIED]: 'Job',
  [ContentJobKind.REEXTRACT]: 'Re-extract',
  [ContentJobKind.EMBED]: 'Embeddings',
  [ContentJobKind.PURGE]: 'Purge',
}

function refreshContent() {
  return Promise.all([
    invalidateService(ContentService),
    invalidateService(VideoService),
    invalidateService(ClipService),
    invalidateService(QueueService),
  ])
}

export function ContentPage() {
  const [tab, setTab] = useState<string>('videos')
  return (
    <PageContentBlock title="Content · Admin · Krill">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <Heading>Content</Heading>
        <SegmentedTabs options={tabs} value={tab} onChange={setTab} />
      </div>
      <Jobs />
      {tab === 'videos' ? <Videos /> : <Clips />}
    </PageContentBlock>
  )
}

function running(j: ContentJob) {
  return j.done + j.failed < j.total
}

function Jobs() {
  const { data } = useQuery(
    ContentService.method.listContentJobs,
    {},
    { refetchInterval: (q) => (q.state.data?.jobs.some(running) ? 2000 : false) },
  )
  const jobs = data?.jobs ?? []
  if (jobs.length === 0) return null
  return (
    <section className="mt-8">
      <Subheading>Jobs</Subheading>
      <ul className="mt-3 divide-y divide-zinc-950/5 dark:divide-white/5">
        {jobs.slice(0, 5).map((j) => {
          const at = j.createdAt ? timestampDate(j.createdAt) : null
          return (
            <li key={String(j.id)} className="flex flex-wrap items-center gap-x-4 gap-y-2 py-3">
              <div className="min-w-0 flex-1">
                <div className="flex items-center gap-2 text-sm/6">
                  <span className="font-medium text-zinc-950 dark:text-white">
                    {jobKinds[j.kind]}
                  </span>
                  <span className="truncate text-zinc-500 dark:text-zinc-400">{j.summary}</span>
                </div>
                <div className="text-xs/5 text-zinc-500 dark:text-zinc-400">
                  {j.createdBy?.name ?? 'Automatic'}
                  {at && ` · ${formatRelative(at)}`}
                </div>
              </div>
              <div className="flex w-full items-center gap-3 sm:w-64">
                <Meter value={j.total ? (j.done + j.failed) / j.total : 1} label={j.summary} />
                <span className="text-xs/5 whitespace-nowrap text-zinc-500 tabular-nums dark:text-zinc-400">
                  {formatNumber(j.done)}/{formatNumber(j.total)}
                </span>
                {j.failed > 0 && <Badge color="red">{formatNumber(j.failed)} failed</Badge>}
              </div>
            </li>
          )
        })}
      </ul>
    </section>
  )
}

function Embeddings({
  clipCount,
  embedded,
  current,
  model,
}: {
  clipCount: number
  embedded: number
  current: number
  model: string
}) {
  if (clipCount === 0) return <span className="text-zinc-400">—</span>
  // Without a known model every embedding counts as current.
  const ok = model ? current : embedded
  const stale = embedded - ok
  return (
    <span className="inline-flex items-center gap-2">
      <span className="tabular-nums">
        {formatNumber(ok)}/{formatNumber(clipCount)}
      </span>
      {stale > 0 && <Badge color="amber">{formatNumber(stale)} stale</Badge>}
    </span>
  )
}

function Videos() {
  const { data, isPending, error } = useQuery(
    ContentService.method.listContentVideos,
    {},
    {
      refetchInterval: (q) =>
        q.state.data?.videos.some(
          (v) => v.status === VideoStatus.QUEUED || v.status === VideoStatus.PROCESSING,
        )
          ? 2000
          : false,
    },
  )
  const videos = data?.videos ?? []
  const selection = useSelection(videos.map((v) => String(v.id)))
  const picked = videos.filter((v) => selection.has(String(v.id)))
  const [reextracting, setReextracting] = useState(false)
  const [embedding, setEmbedding] = useState(false)

  if (isPending) {
    return (
      <div className="flex justify-center py-16">
        <LoadingSpinner />
      </div>
    )
  }
  if (error) return <QueryError title="Could not load videos" error={error} className="mt-8" />
  if (videos.length === 0) {
    return <p className="py-10 text-center text-sm/6 text-zinc-500 dark:text-zinc-400">No videos</p>
  }

  return (
    <>
      <div className="mt-8 flex h-9 items-center gap-3">
        {picked.length > 0 && (
          <>
            <span className="text-sm/6 font-medium text-zinc-950 dark:text-white">
              {formatNumber(picked.length)} selected
            </span>
            <Button outline onClick={() => setReextracting(true)}>
              <ArrowPathIcon data-slot="icon" />
              Re-extract
            </Button>
            <Button outline onClick={() => setEmbedding(true)}>
              <CpuChipIcon data-slot="icon" />
              Embeddings
            </Button>
          </>
        )}
      </div>
      <Table className="mt-2 [--gutter:--spacing(6)] lg:[--gutter:--spacing(10)]">
        <TableHead>
          <TableRow>
            <TableHeader className="w-0">
              <Checkbox
                aria-label="Select all"
                checked={selection.all}
                indeterminate={selection.some}
                onChange={selection.toggleAll}
              />
            </TableHeader>
            <TableHeader>Video</TableHeader>
            <TableHeader>Status</TableHeader>
            <TableHeader className="text-right">Clips</TableHeader>
            <TableHeader className="text-right">Labeled frames</TableHeader>
            <TableHeader className="text-right">Boxes</TableHeader>
            <TableHeader>
              <span title={data.currentModel || undefined}>Embeddings</span>
            </TableHeader>
          </TableRow>
        </TableHead>
        <TableBody>
          {videos.map((v) => (
            <TableRow key={String(v.id)}>
              <TableCell>
                <Checkbox
                  aria-label={`Select ${v.name}`}
                  checked={selection.has(String(v.id))}
                  onChange={(on) => selection.toggle(String(v.id), on)}
                />
              </TableCell>
              <TableCell>
                <Link
                  to={`/videos/${v.id}`}
                  className="font-medium text-zinc-950 hover:underline dark:text-white"
                >
                  {v.name}
                </Link>
              </TableCell>
              <TableCell title={v.error || undefined}>
                <VideoStatusBadge video={v} />
              </TableCell>
              <TableCell className="text-right tabular-nums">{formatNumber(v.clipCount)}</TableCell>
              <TableCell className="text-right tabular-nums">
                {formatNumber(v.labeledFrameCount)}
              </TableCell>
              <TableCell className="text-right tabular-nums">{formatNumber(v.boxCount)}</TableCell>
              <TableCell>
                <Embeddings
                  clipCount={v.clipCount}
                  embedded={v.embeddedClipCount}
                  current={v.currentClipCount}
                  model={data.currentModel}
                />
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
      <ReextractDialog
        videos={reextracting ? picked : []}
        onClose={() => setReextracting(false)}
        onDone={selection.clear}
      />
      <EmbedDialog
        open={embedding}
        videoIds={picked.map((v) => v.id)}
        clipIds={[]}
        model={data.currentModel}
        onClose={() => setEmbedding(false)}
        onDone={selection.clear}
      />
    </>
  )
}

function ReextractDialog({
  videos,
  onClose,
  onDone,
}: {
  videos: ContentVideo[]
  onClose: () => void
  onDone: () => void
}) {
  const open = videos.length > 0
  const videoIds = videos.map((v) => v.id)
  const preview = useQuery(ContentService.method.previewReextract, { videoIds }, { enabled: open })
  // -1 keeps each video's rate.
  const [fps, setFps] = useState(-1)
  const [discard, setDiscard] = useState(false)
  const reextract = useMutation(ContentService.method.reextractVideos, {
    onSuccess: async () => {
      await refreshContent()
      flash.success(`Re-extracting ${plural(videoIds.length, 'video')}`)
      onDone()
      close()
    },
  })
  const close = () => {
    reextract.reset()
    setDiscard(false)
    setFps(-1)
    onClose()
  }
  const impacts = preview.data?.videos ?? []
  const labeled = impacts.filter((v) => v.labeledFrameCount > 0 || v.trackCount > 0)
  const blocked = impacts.filter((v) => v.blocked)
  const sum = (f: (v: (typeof impacts)[number]) => number) => impacts.reduce((n, v) => n + f(v), 0)

  return (
    <Dialog open={open} onClose={close} size="2xl">
      <DialogTitle>Re-extract {plural(videos.length, 'video')}</DialogTitle>
      <DialogBody>
        {preview.isPending ? (
          <div className="flex justify-center py-8">
            <LoadingSpinner />
          </div>
        ) : preview.error ? (
          <QueryError title="Could not count labels" error={preview.error} />
        ) : (
          <>
            <Table dense className="[--gutter:--spacing(1)]">
              <TableHead>
                <TableRow>
                  <TableHeader>Video</TableHeader>
                  <TableHeader className="text-right">Labeled frames</TableHeader>
                  <TableHeader className="text-right">Boxes</TableHeader>
                  <TableHeader className="text-right">Gold</TableHeader>
                  <TableHeader className="text-right">Reviews</TableHeader>
                  <TableHeader className="text-right">Claims</TableHeader>
                </TableRow>
              </TableHead>
              <TableBody>
                {impacts.map((v) => (
                  <TableRow key={String(v.videoId)}>
                    <TableCell className="font-medium">
                      {v.name}
                      {v.blocked && (
                        <Badge color="amber" className="ml-2">
                          {v.blocked}
                        </Badge>
                      )}
                    </TableCell>
                    <TableCell className="text-right tabular-nums">
                      {formatNumber(v.labeledFrameCount)}
                    </TableCell>
                    <TableCell className="text-right tabular-nums">
                      {formatNumber(v.boxCount)}
                    </TableCell>
                    <TableCell className="text-right tabular-nums">
                      {formatNumber(v.goldFrameCount)}
                    </TableCell>
                    <TableCell className="text-right tabular-nums">
                      {formatNumber(v.openReviewCount)}
                    </TableCell>
                    <TableCell className="text-right tabular-nums">
                      {formatNumber(v.claimCount)}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
            <Field className="mt-6">
              <Label>Frame rate</Label>
              <Select value={fps} onChange={(e) => setFps(Number(e.target.value))}>
                <option value={-1}>Keep current</option>
                {extractFpsOptions.map((o) => (
                  <option key={o.value} value={o.value}>
                    {o.label}
                  </option>
                ))}
              </Select>
            </Field>
            {labeled.length > 0 && (
              <CheckboxField className="mt-6">
                <Checkbox checked={discard} onChange={setDiscard} color="red" />
                <Label>
                  Delete{' '}
                  {plural(
                    sum((v) => v.boxCount),
                    'box',
                    'boxes',
                  )}{' '}
                  on{' '}
                  {plural(
                    sum((v) => v.labeledFrameCount),
                    'labeled frame',
                  )}
                </Label>
              </CheckboxField>
            )}
          </>
        )}
        <FormError error={reextract.error} className="mt-6" />
      </DialogBody>
      <DialogActions>
        <Button plain onClick={close}>
          Cancel
        </Button>
        <Button
          color={labeled.length > 0 ? 'red' : 'sky'}
          disabled={
            preview.isPending ||
            blocked.length > 0 ||
            (labeled.length > 0 && !discard) ||
            reextract.isPending
          }
          onClick={() =>
            reextract.mutate({
              videoIds,
              discardLabels: discard,
              extractFps: fps < 0 ? undefined : fps,
            })
          }
        >
          Re-extract
        </Button>
      </DialogActions>
    </Dialog>
  )
}

function EmbedDialog({
  open,
  videoIds,
  clipIds,
  model,
  onClose,
  onDone,
}: {
  open: boolean
  videoIds: bigint[]
  clipIds: bigint[]
  model: string
  onClose: () => void
  onDone: () => void
}) {
  const [onlyStale, setOnlyStale] = useState(true)
  const regenerate = useMutation(ContentService.method.regenerateEmbeddings, {
    onSuccess: async (res) => {
      await refreshContent()
      flash.success(res.clips > 0 ? `Embedding ${plural(res.clips, 'clip')}` : 'Nothing to embed')
      onDone()
      close()
    },
  })
  const close = () => {
    regenerate.reset()
    onClose()
  }
  const count = videoIds.length ? plural(videoIds.length, 'video') : plural(clipIds.length, 'clip')

  return (
    <Dialog open={open} onClose={close} size="md">
      <DialogTitle>Embeddings for {count}</DialogTitle>
      <DialogBody>
        <dl className="text-sm/6">
          <dt className="text-zinc-500 dark:text-zinc-400">Model</dt>
          <dd className="font-mono text-zinc-950 dark:text-white">{model || 'Unknown'}</dd>
        </dl>
        <CheckboxField className="mt-6">
          <Checkbox checked={onlyStale} onChange={setOnlyStale} color="sky" />
          <Label>Only missing or stale</Label>
        </CheckboxField>
        <FormError error={regenerate.error} className="mt-6" />
      </DialogBody>
      <DialogActions>
        <Button plain onClick={close}>
          Cancel
        </Button>
        <Button
          color="sky"
          disabled={regenerate.isPending}
          onClick={() => regenerate.mutate({ videoIds, clipIds, onlyStale })}
        >
          Queue
        </Button>
      </DialogActions>
    </Dialog>
  )
}

function Clips() {
  const [videoId, setVideoId] = useState('')
  const [claimantId, setClaimantId] = useState('')
  const videos = useQuery(ContentService.method.listContentVideos, {})
  const users = useQuery(UserService.method.listUsers, {})
  const { data, isPending, error } = useQuery(ContentService.method.listContentClips, {
    videoId: videoId ? BigInt(videoId) : 0n,
    claimantId,
  })
  const clips = data?.clips ?? []
  const selection = useSelection(clips.map((c) => String(c.id)))
  const picked = clips.filter((c) => selection.has(String(c.id)))
  const [assigning, setAssigning] = useState(false)
  const [embedding, setEmbedding] = useState(false)

  const reassign = useMutation(ContentService.method.reassignClips, {
    onSuccess: async (res, req) => {
      await refreshContent()
      flash.success(
        req.userId
          ? `Assigned ${plural(res.clips, 'clip')}`
          : `Released ${plural(res.clips, 'clip')}`,
      )
      selection.clear()
      setAssigning(false)
    },
    onError: (err) => flash.error('Could not reassign clips', err),
  })

  return (
    <>
      <div className="mt-8 flex flex-wrap gap-3">
        <Select
          aria-label="Video"
          value={videoId}
          onChange={(e) => setVideoId(e.target.value)}
          className="sm:w-64"
        >
          <option value="">Any video</option>
          {videos.data?.videos.map((v) => (
            <option key={String(v.id)} value={String(v.id)}>
              {v.name}
            </option>
          ))}
        </Select>
        <Select
          aria-label="Claimed by"
          value={claimantId}
          onChange={(e) => setClaimantId(e.target.value)}
          className="sm:w-48"
        >
          <option value="">{videoId ? 'Anyone or no one' : 'Anyone'}</option>
          {users.data?.users.map((u) => (
            <option key={u.id} value={u.id}>
              {u.name}
            </option>
          ))}
        </Select>
      </div>

      <div className="mt-4 flex h-9 items-center gap-3">
        {picked.length > 0 && (
          <>
            <span className="text-sm/6 font-medium text-zinc-950 dark:text-white">
              {formatNumber(picked.length)} selected
            </span>
            <Button outline onClick={() => setAssigning(true)}>
              <UserPlusIcon data-slot="icon" />
              Assign
            </Button>
            <Button
              outline
              disabled={reassign.isPending || !picked.some((c) => c.claimant)}
              onClick={() => reassign.mutate({ clipIds: picked.map((c) => c.id), userId: '' })}
            >
              <UserMinusIcon data-slot="icon" />
              Release
            </Button>
            <Button outline onClick={() => setEmbedding(true)}>
              <CpuChipIcon data-slot="icon" />
              Embeddings
            </Button>
          </>
        )}
      </div>

      {isPending ? (
        <div className="flex justify-center py-16">
          <LoadingSpinner />
        </div>
      ) : error ? (
        <QueryError title="Could not load clips" error={error} className="mt-4" />
      ) : clips.length === 0 ? (
        <p className="py-10 text-center text-sm/6 text-zinc-500 dark:text-zinc-400">
          {videoId || claimantId ? 'No matching clips' : 'No claimed clips'}
        </p>
      ) : (
        <Table className="mt-2 [--gutter:--spacing(6)] lg:[--gutter:--spacing(10)]">
          <TableHead>
            <TableRow>
              <TableHeader className="w-0">
                <Checkbox
                  aria-label="Select all"
                  checked={selection.all}
                  indeterminate={selection.some}
                  onChange={selection.toggleAll}
                />
              </TableHeader>
              <TableHeader>Clip</TableHeader>
              <TableHeader>Claimed by</TableHeader>
              <TableHeader className="text-right">Labeled</TableHeader>
              <TableHeader className="text-right">Boxes</TableHeader>
              <TableHeader>Embeddings</TableHeader>
            </TableRow>
          </TableHead>
          <TableBody>
            {clips.map((c) => (
              <ClipRow
                key={String(c.id)}
                clip={c}
                model={data.currentModel}
                checked={selection.has(String(c.id))}
                onCheck={(on) => selection.toggle(String(c.id), on)}
              />
            ))}
          </TableBody>
        </Table>
      )}
      {data?.truncated && (
        <p className="mt-4 text-center text-sm/6 text-zinc-500 dark:text-zinc-400">
          First {formatNumber(clips.length)}
        </p>
      )}

      <AssignDialog
        open={assigning}
        count={picked.length}
        busy={reassign.isPending}
        error={reassign.error}
        onClose={() => {
          reassign.reset()
          setAssigning(false)
        }}
        onAssign={(userId) => reassign.mutate({ clipIds: picked.map((c) => c.id), userId })}
      />
      <EmbedDialog
        open={embedding}
        videoIds={[]}
        clipIds={picked.map((c) => c.id)}
        model={data?.currentModel ?? ''}
        onClose={() => setEmbedding(false)}
        onDone={selection.clear}
      />
    </>
  )
}

function ClipRow({
  clip: c,
  model,
  checked,
  onCheck,
}: {
  clip: ContentClip
  model: string
  checked: boolean
  onCheck: (on: boolean) => void
}) {
  const activeAt = c.claimActiveAt ? timestampDate(c.claimActiveAt) : null
  const name = `${c.videoName} · Clip ${c.index + 1}`
  return (
    <TableRow>
      <TableCell>
        <Checkbox aria-label={`Select ${name}`} checked={checked} onChange={onCheck} />
      </TableCell>
      <TableCell>
        <Link
          to={`/clips/${c.id}`}
          className="font-medium text-zinc-950 hover:underline dark:text-white"
        >
          {name}
        </Link>
      </TableCell>
      <TableCell>
        {c.claimant ? (
          <div className="flex items-center gap-2">
            <Avatar seed={c.claimant.id} className="size-6" />
            <div>
              <div className="text-zinc-950 dark:text-white">{c.claimant.name}</div>
              <div className="text-xs/5 text-zinc-500 dark:text-zinc-400">
                {activeAt && formatRelative(activeAt)}
                {!c.claimActive && ' · idle'}
              </div>
            </div>
          </div>
        ) : (
          <span className="text-zinc-400">—</span>
        )}
      </TableCell>
      <TableCell className="text-right tabular-nums">
        {formatNumber(c.labeledFrameCount)}/{formatNumber(c.frameCount)}
      </TableCell>
      <TableCell className="text-right tabular-nums">{formatNumber(c.boxCount)}</TableCell>
      <TableCell>
        {!c.embedModel ? (
          <span className="text-zinc-400">—</span>
        ) : model && c.embedModel !== model ? (
          <Badge color="amber" title={c.embedModel}>
            Stale
          </Badge>
        ) : (
          <Badge color="green" title={c.embedModel}>
            Current
          </Badge>
        )}
      </TableCell>
    </TableRow>
  )
}

function AssignDialog({
  open,
  count,
  busy,
  error,
  onClose,
  onAssign,
}: {
  open: boolean
  count: number
  busy: boolean
  error: unknown
  onClose: () => void
  onAssign: (userId: string) => void
}) {
  const users = useQuery(UserService.method.listUsers, {}, { enabled: open })
  const [userId, setUserId] = useState('')
  const active = (users.data?.users ?? []).filter((u) => !u.disabled)
  return (
    <Dialog open={open} onClose={onClose} size="md">
      <form
        onSubmit={(e) => {
          e.preventDefault()
          if (userId) onAssign(userId)
        }}
      >
        <DialogTitle>Assign {plural(count, 'clip')}</DialogTitle>
        <DialogBody>
          <Field>
            <Label>Labeler</Label>
            <Select value={userId} onChange={(e) => setUserId(e.target.value)}>
              <option value="" disabled>
                Choose
              </option>
              {active.map((u) => (
                <option key={u.id} value={u.id}>
                  {u.name} (@{u.username})
                </option>
              ))}
            </Select>
          </Field>
          <FormError error={error} className="mt-6" />
        </DialogBody>
        <DialogActions>
          <Button plain onClick={onClose}>
            Cancel
          </Button>
          <Button type="submit" color="sky" disabled={!userId || busy}>
            Assign
          </Button>
        </DialogActions>
      </form>
    </Dialog>
  )
}
