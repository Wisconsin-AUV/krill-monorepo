import { useEffect } from 'react'
import { annotationClient, clipClient } from '@/lib/clients'
import { flash } from '@/lib/flash'
import { useLabelStore } from './useLabelStore'

const POLL_MS = 1500

// Boxes stream in while the GPU worker tracks, so the clip is polled until
// no track is left tracking and no exemplar search is left running.
export function useTrackingPoll(clipId: bigint | undefined) {
  const active = useLabelStore(
    (s) => Object.keys(s.tracking).length > 0 || s.exemplarSearches.length > 0,
  )

  useEffect(() => {
    if (!active || clipId === undefined) return
    let stopped = false
    let timer: ReturnType<typeof setTimeout>
    const poll = async () => {
      try {
        for (const search of useLabelStore.getState().exemplarSearches) {
          const res = await annotationClient.getExemplarSearch({ id: search.id })
          if (stopped) return
          if (!res.done) continue
          useLabelStore.getState().finishExemplarSearch(search.id, res.trackIds)
          if (res.error) flash.error('Could not find similar objects', res.error)
          else if (res.trackIds.length === 0) flash.success('No similar objects found')
        }
        if (Object.keys(useLabelStore.getState().tracking).length > 0) {
          const clip = await clipClient.getClip({ id: clipId })
          if (stopped) return
          useLabelStore.getState().refreshTracking(clip)
        }
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
