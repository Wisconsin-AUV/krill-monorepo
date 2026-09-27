import { useEffect } from 'react'
import { clipClient } from '@/lib/clients'
import { flash } from '@/lib/flash'
import { useLabelStore } from './useLabelStore'

const POLL_MS = 1500

// Boxes stream in while the GPU worker tracks, so the clip is polled until
// no track is left tracking.
export function useTrackingPoll(clipId: bigint | undefined) {
  const active = useLabelStore((s) => Object.keys(s.tracking).length > 0)

  useEffect(() => {
    if (!active || clipId === undefined) return
    let stopped = false
    let timer: ReturnType<typeof setTimeout>
    const poll = async () => {
      try {
        const clip = await clipClient.getClip({ id: clipId })
        if (stopped) return
        useLabelStore.getState().refreshTracking(clip)
      } catch (err) {
        if (!stopped) flash.error('Could not load tracked boxes', err)
        return
      }
      timer = setTimeout(poll, POLL_MS)
    }
    timer = setTimeout(poll, POLL_MS)
    return () => {
      stopped = true
      clearTimeout(timer)
    }
  }, [active, clipId])
}
