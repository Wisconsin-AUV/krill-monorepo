import { useMutation, useQuery } from '@connectrpc/connect-query'
import { timestampDate } from '@bufbuild/protobuf/wkt'
import {
  ArrowPathIcon,
  ChevronLeftIcon,
  EllipsisHorizontalIcon,
  ExclamationTriangleIcon,
  PencilSquareIcon,
  TrashIcon,
} from '@heroicons/react/20/solid'
import { useState } from 'react'
import { Link, useNavigate, useParams, useSearchParams } from 'react-router'
import { ConfirmAlert } from '@/components/ConfirmAlert'
import { EditVideoDialog } from '@/components/EditVideoDialog'
import { Meter } from '@/components/Meter'
import { SegmentedTabs } from '@/components/SegmentedTabs'
import { Thumbnail } from '@/components/Thumbnail'
import { VideoStatusBadge } from '@/components/VideoStatusBadge'
import { Button } from '@/components/ui/Button'
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
import { Heading, Subheading } from '@/components/ui/Heading'
import LoadingSpinner from '@/components/ui/LoadingSpinner'
import PageContentBlock from '@/components/ui/PageContentBlock'
import { Text } from '@/components/ui/Text'
import { Permission } from '@/gen/krill/v1/user_pb'
import { VideoService, VideoStatus, type Clip } from '@/gen/krill/v1/video_pb'
import { useCan, useUser } from '@/lib/auth'
import { errorMessage } from '@/lib/errors'
import { flash } from '@/lib/flash'
import { formatDuration, formatFps, formatNumber, formatRelative, plural } from '@/lib/format'
import { invalidateService } from '@/lib/queryClient'
import { clipState } from '@/lib/queue'
import { extractFpsOptions, isIngesting, splitLabel } from '@/lib/video'
import { NotFound } from './NotFound'

const filters = [
  { key: 'all', label: 'All' },
  { key: 'todo', label: 'Not started' },
  { key: 'progress', label: 'In progress' },
  { key: 'done', label: 'Done' },
  { key: 'mine', label: 'Mine' },
] as const

function matches(filter: string, clip: Clip, userId: string | undefined) {
  if (filter === 'mine') return clip.claim?.user?.id === userId
  if (filter === 'all') return true
  return clipState(clip.labeledFrameCount, clip.frameCount) === filter
}

function ClipCard({ clip, userId }: { clip: Clip; userId: string | undefined }) {
  const start = Number(clip.startMs)
  const labeler =
    clip.claim?.active && clip.labeledFrameCount < clip.frameCount
      ? clip.claim.user?.id === userId
        ? 'You are labeling'
        : `${clip.claim.user?.name} is labeling`
      : undefined
  return (
    <Link
      to={`/clips/${clip.id}`}
      className="group rounded-lg focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-sky-500"
    >
      <Thumbnail
        src={clip.thumbnailUrl}
        alt=""
        className="transition group-hover:opacity-90 group-hover:ring-2 group-hover:ring-sky-500"
      />
      <div className="mt-2 flex items-baseline justify-between gap-2">
        <span className="text-sm/6 font-medium text-zinc-950 dark:text-white">
          Clip {clip.index + 1}
        </span>
        <span className="text-xs/6 text-zinc-500 tabular-nums dark:text-zinc-400">
          {formatDuration(start)}–{formatDuration(start + Number(clip.durationMs))}
        </span>
      </div>
      <div className="text-xs/5 text-zinc-500 tabular-nums dark:text-zinc-400">
        {formatNumber(clip.labeledFrameCount)} of {plural(clip.frameCount, 'frame')} done ·{' '}
        {plural(clip.boxCount, 'box', 'boxes')}
      </div>
      <Meter
        value={clip.frameCount > 0 ? clip.labeledFrameCount / clip.frameCount : 0}
        label={`Clip ${clip.index + 1} progress`}
        className="mt-2"
      />
      {labeler && <div className="mt-2 text-xs/5 text-sky-600 dark:text-sky-400">{labeler}</div>}
    </Link>
  )
}

export function VideoPage() {
  const params = useParams()
  const navigate = useNavigate()
  const id = /^\d+$/.test(params.id ?? '') ? BigInt(params.id!) : undefined
  const [editing, setEditing] = useState(false)
  const [deleting, setDeleting] = useState(false)
  const canManage = useCan(Permission.MANAGE_VIDEOS)
  const userId = useUser()?.id
  const [search, setSearch] = useSearchParams()
  const filter = filters.find((f) => f.key === search.get('show'))?.key ?? 'all'

  const { data, isPending, error } = useQuery(
    VideoService.method.getVideo,
    { id },
    {
      enabled: id !== undefined,
      refetchInterval: (q) =>
        q.state.data?.video && isIngesting(q.state.data.video) ? 1500 : false,
    },
  )
  const retry = useMutation(VideoService.method.startIngest, {
    onSuccess: () => invalidateService(VideoService),
    onError: (err) => flash.error('Could not restart ingest', err),
  })
  const remove = useMutation(VideoService.method.deleteVideo, {
    onSuccess: async () => {
      await invalidateService(VideoService)
      flash.success('Video deleted')
      navigate('/videos')
    },
    onError: (err) => flash.error('Could not delete video', err),
  })

  if (id === undefined) return <NotFound />
  if (isPending) {
    return (
      <div className="flex justify-center py-24">
        <LoadingSpinner />
      </div>
    )
  }
  if (error || !data.video) {
    return error && error.code !== 5 ? (
      <PageContentBlock>
        <Text className="text-red-600 dark:text-red-400">
          Could not load video: {errorMessage(error)}
        </Text>
      </PageContentBlock>
    ) : (
      <NotFound />
    )
  }

  const video = data.video
  const clips = data.clips.filter((c) => matches(filter, c, userId))
  const extractLabel =
    video.extractFps > 0
      ? (extractFpsOptions.find((o) => o.value === video.extractFps)?.label ??
        formatFps(video.extractFps))
      : 'Native'

  return (
    <PageContentBlock title={`${video.name} · Krill`}>
      <Link
        to={canManage ? '/videos' : '/'}
        className="inline-flex items-center gap-2 text-sm/6 text-zinc-500 hover:text-zinc-700 dark:text-zinc-400 dark:hover:text-zinc-200"
      >
        <ChevronLeftIcon className="size-4 fill-zinc-400 dark:fill-zinc-500" />
        {canManage ? 'Videos' : 'Home'}
      </Link>

      <div className="mt-4 flex flex-wrap items-center justify-between gap-4">
        <div className="flex min-w-0 flex-wrap items-center gap-x-4 gap-y-2">
          <Heading className="truncate">{video.name}</Heading>
          <VideoStatusBadge video={video} />
        </div>
        {canManage && (
          <div className="flex gap-3">
            <Button outline onClick={() => setEditing(true)}>
              <PencilSquareIcon data-slot="icon" />
              Edit
            </Button>
            <Dropdown>
              <DropdownButton plain aria-label="More options">
                <EllipsisHorizontalIcon data-slot="icon" />
              </DropdownButton>
              <DropdownMenu anchor="bottom end">
                {video.status === VideoStatus.FAILED && (
                  <DropdownItem onClick={() => retry.mutate({ videoId: video.id })}>
                    <ArrowPathIcon data-slot="icon" />
                    <DropdownLabel>Retry ingest</DropdownLabel>
                  </DropdownItem>
                )}
                <DropdownItem onClick={() => setDeleting(true)}>
                  <TrashIcon data-slot="icon" />
                  <DropdownLabel>Delete video</DropdownLabel>
                </DropdownItem>
              </DropdownMenu>
            </Dropdown>
          </div>
        )}
      </div>

      {video.status === VideoStatus.FAILED && (
        <div className="mt-6 flex gap-3 rounded-xl bg-red-50 p-4 ring-1 ring-red-600/10 dark:bg-red-500/10 dark:ring-red-500/20">
          <ExclamationTriangleIcon className="size-5 shrink-0 text-red-500" aria-hidden="true" />
          <div className="min-w-0 flex-1">
            <p className="text-sm/6 font-medium text-red-800 dark:text-red-300">
              Frame extraction failed
            </p>
            <p className="mt-1 text-sm/6 break-words text-red-700 dark:text-red-400">
              {video.error}
            </p>
          </div>
          {canManage && (
            <Button
              color="red"
              onClick={() => retry.mutate({ videoId: video.id })}
              disabled={retry.isPending}
            >
              Retry
            </Button>
          )}
        </div>
      )}

      {video.status === VideoStatus.UPLOADING && (
        <div className="mt-6 rounded-xl bg-zinc-50 p-4 text-sm/6 text-zinc-600 ring-1 ring-zinc-950/5 dark:bg-white/5 dark:text-zinc-300 dark:ring-white/10">
          This video has not finished uploading. If the upload was interrupted, delete it and upload
          again.
        </div>
      )}

      {isIngesting(video) && (
        <div className="mt-6 rounded-xl bg-sky-50 p-4 ring-1 ring-sky-600/10 dark:bg-sky-500/10 dark:ring-sky-500/20">
          <div className="flex items-baseline justify-between text-sm/6">
            <span className="font-medium text-sky-900 dark:text-sky-200">
              {video.status === VideoStatus.QUEUED
                ? 'Waiting to extract frames'
                : 'Extracting frames'}
            </span>
            <span className="text-sky-700 tabular-nums dark:text-sky-300">
              {Math.round(video.ingestProgress * 100)}%
            </span>
          </div>
          <Meter value={video.ingestProgress} label="Ingest progress" className="mt-2" />
        </div>
      )}

      <DescriptionList className="mt-10">
        <DescriptionTerm>Filename</DescriptionTerm>
        <DescriptionDetails className="break-all">{video.filename}</DescriptionDetails>
        <DescriptionTerm>Added</DescriptionTerm>
        <DescriptionDetails>
          {video.createdAt && formatRelative(timestampDate(video.createdAt))}
        </DescriptionDetails>
        {video.status === VideoStatus.READY && (
          <>
            <DescriptionTerm>Resolution</DescriptionTerm>
            <DescriptionDetails className="tabular-nums">
              {video.width} × {video.height}
            </DescriptionDetails>
            <DescriptionTerm>Duration</DescriptionTerm>
            <DescriptionDetails className="tabular-nums">
              {formatDuration(video.durationMs)}
            </DescriptionDetails>
            <DescriptionTerm>Frames</DescriptionTerm>
            <DescriptionDetails className="tabular-nums">
              {formatNumber(video.frameCount)} at {formatFps(video.fps)}
            </DescriptionDetails>
            <DescriptionTerm>Labeled</DescriptionTerm>
            <DescriptionDetails className="tabular-nums">
              {plural(video.labeledFrameCount, 'frame')} done ·{' '}
              {plural(video.boxCount, 'box', 'boxes')}
            </DescriptionDetails>
          </>
        )}
        {canManage && (
          <>
            <DescriptionTerm>Extraction rate</DescriptionTerm>
            <DescriptionDetails>{extractLabel}</DescriptionDetails>
            <DescriptionTerm>Dataset split</DescriptionTerm>
            <DescriptionDetails>{splitLabel(video.split)}</DescriptionDetails>
          </>
        )}
        <DescriptionTerm>Notes</DescriptionTerm>
        <DescriptionDetails className="whitespace-pre-line">
          {video.notes || <span className="text-zinc-400 dark:text-zinc-500">None</span>}
        </DescriptionDetails>
      </DescriptionList>

      {data.clips.length > 0 && (
        <>
          <div className="mt-12 flex flex-wrap items-center justify-between gap-4">
            <Subheading>
              Clips{' '}
              <span className="font-normal text-zinc-500 dark:text-zinc-400">{clips.length}</span>
            </Subheading>
            <SegmentedTabs
              options={filters}
              value={filter}
              onChange={(key) => setSearch(key === 'all' ? {} : { show: key }, { replace: true })}
            />
          </div>
          {clips.length === 0 ? (
            <Text className="mt-6">No clips.</Text>
          ) : (
            <div className="mt-4 grid grid-cols-2 gap-x-6 gap-y-8 sm:grid-cols-3 lg:grid-cols-4">
              {clips.map((c) => (
                <ClipCard key={String(c.id)} clip={c} userId={userId} />
              ))}
            </div>
          )}
        </>
      )}

      <EditVideoDialog video={video} open={editing} onClose={() => setEditing(false)} />
      <ConfirmAlert
        open={deleting}
        onClose={() => setDeleting(false)}
        onConfirm={() => remove.mutate({ id: video.id })}
        busy={remove.isPending}
        title={`Delete ${video.name}?`}
        description="This removes the video, its frames, and every label on them. It cannot be undone."
        confirmLabel="Delete video"
      />
    </PageContentBlock>
  )
}
