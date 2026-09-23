import { createQueryOptions } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import { useEffect } from 'react'
import { ClipService, type GetClipResponse } from '@/gen/krill/v1/clip_pb'
import { transport } from '@/lib/transport'
import { loadFrame } from './imageCache'

const FRAMES_TO_WARM = 4
const PREFETCH_STALE_MS = 30_000

// Fetches the previous and next clips and their first frames so switching
// clips shows the new frame immediately instead of waiting on the network.
export function usePrefetchNeighbours(clip: GetClipResponse | undefined) {
  const queryClient = useQueryClient()

  useEffect(() => {
    if (!clip) return
    for (const id of [clip.nextClipId, clip.previousClipId]) {
      if (!id) continue
      queryClient
        .fetchQuery({
          ...createQueryOptions(ClipService.method.getClip, { id }, { transport }),
          staleTime: PREFETCH_STALE_MS,
        })
        .then((next) => next.frames.slice(0, FRAMES_TO_WARM).forEach((f) => loadFrame(f.id, f.url)))
        .catch(() => undefined)
    }
  }, [clip, queryClient])
}
