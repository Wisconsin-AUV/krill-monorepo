import { useEffect } from 'react'
import { isFrameLoaded } from './imageCache'
import { useWorkspaceStore } from './useWorkspaceStore'

// Advances one frame per 1/fps seconds, waiting on frames that have not
// loaded yet so playback never skips a frame the labeler has not seen.
export function usePlayback(fps: number) {
  const playing = useWorkspaceStore((s) => s.playing)

  useEffect(() => {
    if (!playing || fps <= 0) return
    const interval = 1000 / fps
    let last = performance.now()
    let raf = 0
    const tick = (now: number) => {
      const { frames, index, step, setPlaying } = useWorkspaceStore.getState()
      if (index >= frames.length - 1) {
        setPlaying(false)
        return
      }
      if (now - last >= interval && isFrameLoaded(frames[index + 1].id)) {
        last = now
        step(1)
      }
      raf = requestAnimationFrame(tick)
    }
    raf = requestAnimationFrame(tick)
    return () => cancelAnimationFrame(raf)
  }, [playing, fps])
}
