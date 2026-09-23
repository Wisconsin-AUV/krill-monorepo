import { useQuery } from '@connectrpc/connect-query'
import { ArrowUpTrayIcon } from '@heroicons/react/20/solid'
import { FilmIcon } from '@heroicons/react/24/outline'
import { useState } from 'react'
import { EmptyState } from '@/components/EmptyState'
import { Meter } from '@/components/Meter'
import { Stat } from '@/components/Stat'
import { Thumbnail } from '@/components/Thumbnail'
import { UploadDialog } from '@/components/UploadDialog'
import { VideoStatusBadge } from '@/components/VideoStatusBadge'
import { Button } from '@/components/ui/Button'
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
import { Text } from '@/components/ui/Text'
import { Role } from '@/gen/krill/v1/user_pb'
import { VideoService, VideoStatus } from '@/gen/krill/v1/video_pb'
import { timestampDate } from '@bufbuild/protobuf/wkt'
import { useHasRole } from '@/lib/auth'
import { errorMessage } from '@/lib/errors'
import { formatDuration, formatNumber, formatRelative, plural } from '@/lib/format'
import { isIngesting, splitLabel } from '@/lib/video'

export function VideosPage() {
  const [uploading, setUploading] = useState(false)
  const canManage = useHasRole(Role.DEVELOPER)
  const { data, isPending, error } = useQuery(
    VideoService.method.listVideos,
    {},
    { refetchInterval: (q) => (q.state.data?.videos.some(isIngesting) ? 1500 : false) },
  )
  const videos = data?.videos ?? []
  const ready = videos.filter((v) => v.status === VideoStatus.READY)
  const totalFrames = ready.reduce((n, v) => n + v.frameCount, 0)
  const totalClips = ready.reduce((n, v) => n + v.clipCount, 0)
  const totalMs = ready.reduce((n, v) => n + Number(v.durationMs), 0)

  return (
    <PageContentBlock title="Videos · Krill">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <Heading>Videos</Heading>
          <Text className="mt-1">Pool footage to label. Each video is split into short clips.</Text>
        </div>
        {canManage && (
          <Button color="sky" onClick={() => setUploading(true)}>
            <ArrowUpTrayIcon data-slot="icon" />
            Upload videos
          </Button>
        )}
      </div>

      <div className="mt-8 grid gap-8 sm:grid-cols-2 xl:grid-cols-4">
        <Stat title="Videos" value={formatNumber(videos.length)} detail={`${ready.length} ready`} />
        <Stat title="Footage" value={formatDuration(totalMs)} detail="Total ready duration" />
        <Stat title="Clips" value={formatNumber(totalClips)} detail="10 to 20 seconds each" />
        <Stat
          title="Frames labeled"
          value={formatNumber(ready.reduce((n, v) => n + v.labeledFrameCount, 0))}
          detail={`of ${formatNumber(totalFrames)} extracted`}
        />
      </div>

      <Subheading className="mt-14">All videos</Subheading>
      {isPending ? (
        <div className="flex justify-center py-16">
          <LoadingSpinner />
        </div>
      ) : error ? (
        <Text className="mt-4 text-red-600 dark:text-red-400">
          Could not load videos: {errorMessage(error)}
        </Text>
      ) : videos.length === 0 ? (
        <div className="mt-4">
          <EmptyState
            icon={FilmIcon}
            title="No videos yet"
            description={
              canManage
                ? 'Upload pool footage to start labeling.'
                : 'A developer needs to upload footage before you can label.'
            }
          >
            {canManage && (
              <Button color="sky" onClick={() => setUploading(true)}>
                <ArrowUpTrayIcon data-slot="icon" />
                Upload videos
              </Button>
            )}
          </EmptyState>
        </div>
      ) : (
        <Table className="mt-4 [--gutter:--spacing(6)] lg:[--gutter:--spacing(10)]">
          <TableHead>
            <TableRow>
              <TableHeader>Video</TableHeader>
              <TableHeader>Status</TableHeader>
              <TableHeader className="text-right">Duration</TableHeader>
              <TableHeader className="text-right">Frames</TableHeader>
              <TableHeader className="text-right">Clips</TableHeader>
              <TableHeader>Labeled</TableHeader>
              <TableHeader>Split</TableHeader>
              <TableHeader className="text-right">Added</TableHeader>
            </TableRow>
          </TableHead>
          <TableBody>
            {videos.map((v) => (
              <TableRow key={String(v.id)} href={`/videos/${v.id}`} title={v.name}>
                <TableCell>
                  <div className="flex items-center gap-4">
                    <Thumbnail src={v.thumbnailUrl} className="w-24 shrink-0" />
                    <div className="min-w-0">
                      <div className="truncate font-medium text-zinc-950 dark:text-white">
                        {v.name}
                      </div>
                      <div className="truncate text-zinc-500 dark:text-zinc-400">{v.filename}</div>
                    </div>
                  </div>
                </TableCell>
                <TableCell>
                  <VideoStatusBadge video={v} />
                  {v.status === VideoStatus.PROCESSING && (
                    <Meter
                      value={v.ingestProgress}
                      label={`Processing ${v.name}`}
                      className="mt-2 w-24"
                    />
                  )}
                </TableCell>
                <TableCell className="text-right tabular-nums">
                  {v.status === VideoStatus.READY ? formatDuration(v.durationMs) : '—'}
                </TableCell>
                <TableCell className="text-right tabular-nums">
                  {v.status === VideoStatus.READY ? formatNumber(v.frameCount) : '—'}
                </TableCell>
                <TableCell className="text-right tabular-nums">
                  {v.status === VideoStatus.READY ? formatNumber(v.clipCount) : '—'}
                </TableCell>
                <TableCell>
                  {v.status === VideoStatus.READY ? (
                    <div className="w-28">
                      <div className="text-xs/5 text-zinc-500 tabular-nums dark:text-zinc-400">
                        {v.frameCount > 0
                          ? Math.round((v.labeledFrameCount / v.frameCount) * 100)
                          : 0}
                        % · {plural(v.boxCount, 'box', 'boxes')}
                      </div>
                      <Meter
                        value={v.frameCount > 0 ? v.labeledFrameCount / v.frameCount : 0}
                        label={`${v.name} labeling progress`}
                        className="mt-1"
                      />
                    </div>
                  ) : (
                    '—'
                  )}
                </TableCell>
                <TableCell className="text-zinc-500 dark:text-zinc-400">
                  {splitLabel(v.split)}
                </TableCell>
                <TableCell className="text-right text-zinc-500 dark:text-zinc-400">
                  {v.createdAt && formatRelative(timestampDate(v.createdAt))}
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}

      <UploadDialog open={uploading} onClose={() => setUploading(false)} />
    </PageContentBlock>
  )
}
