import { JobStatus } from '@/gen/krill/v1/storage_pb'

export function isActive(status: JobStatus | undefined) {
  return status === JobStatus.QUEUED || status === JobStatus.RUNNING
}
