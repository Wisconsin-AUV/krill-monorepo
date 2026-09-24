import { useMutation } from '@connectrpc/connect-query'
import { useNavigate } from 'react-router'
import { QueueService } from '@/gen/krill/v1/queue_pb'
import type { ClipClaim } from '@/gen/krill/v1/video_pb'
import { flash } from '@/lib/flash'

export type ClipState = 'todo' | 'progress' | 'done'

export function clipState(labeledFrames: number, frames: number): ClipState {
  if (frames > 0 && labeledFrames >= frames) return 'done'
  return labeledFrames > 0 ? 'progress' : 'todo'
}

// Someone else is actively on the clip, so starting it would duplicate work.
export function takenByOther(claim: ClipClaim | undefined, userId: string | undefined): boolean {
  return !!claim?.active && claim.user?.id !== userId
}

export function useClaimNextClip() {
  const navigate = useNavigate()
  return useMutation(QueueService.method.claimNextClip, {
    onSuccess: ({ clipId }) => {
      if (clipId) navigate(`/clips/${clipId}`)
      else flash.success('Nothing left to label', 'Every clip is finished or being labeled.')
    },
    onError: (err) => flash.error('Could not get the next clip', err),
  })
}
