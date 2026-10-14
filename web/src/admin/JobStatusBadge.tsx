import { Badge } from '@/components/ui/Badge'
import { JobStatus } from '@/gen/krill/v1/storage_pb'

export function JobStatusBadge({ status }: { status: JobStatus }) {
  switch (status) {
    case JobStatus.QUEUED:
      return <Badge>Queued</Badge>
    case JobStatus.RUNNING:
      return <Badge color="sky">Running</Badge>
    case JobStatus.DONE:
      return <Badge color="green">Done</Badge>
    case JobStatus.FAILED:
      return <Badge color="red">Failed</Badge>
    default:
      return null
  }
}
