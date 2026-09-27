import { Badge } from '@/components/ui/Badge'
import { VideoStatus, type Video } from '@/gen/krill/v1/video_pb'
import { statusMeta } from '@/lib/video'

export function VideoStatusBadge({ video }: { video: Video }) {
  const { label, color } = statusMeta[video.status]
  return (
    <Badge color={color}>
      {video.status === VideoStatus.PROCESSING && (
        <span className="size-1.5 animate-pulse rounded-full bg-current" aria-hidden="true" />
      )}
      {label}
      {video.status === VideoStatus.PROCESSING && (
        <span className="tabular-nums">{Math.round(video.ingestProgress * 100)}%</span>
      )}
    </Badge>
  )
}
