import { ModelTrialStatus, type ModelTrial } from '@/gen/krill/v1/pipeline_pb'

export interface TrialScore {
  matched: number
  missed: number
  extra: number
  precision: number | undefined
  recall: number | undefined
}

export interface VideoTrials {
  videoId: bigint
  videoName: string
  // In clip order.
  trials: ModelTrial[]
  done: number
  queued: number
  running: number
  failed: number
  framesDone: number
  framesTotal: number
  score: TrialScore | undefined
}

// Pools the counts across clips instead of averaging each clip's ratios, so
// small clips don't skew the score.
export function scoreTrials(trials: ModelTrial[]): TrialScore | undefined {
  let matched = 0
  let missed = 0
  let extra = 0
  let any = false
  for (const t of trials) {
    const all = t.summary?.all
    if (t.status !== ModelTrialStatus.SUCCEEDED || !all) continue
    any = true
    matched += all.matched
    missed += all.missed
    extra += all.extra
  }
  if (!any) return undefined
  return {
    matched,
    missed,
    extra,
    precision: matched + extra > 0 ? matched / (matched + extra) : undefined,
    recall: matched + missed > 0 ? matched / (matched + missed) : undefined,
  }
}

// Expects trials newest first, as ListModelTrials returns them, and keeps
// videos in that order.
export function groupTrials(trials: ModelTrial[]): VideoTrials[] {
  const byVideo = new Map<bigint, ModelTrial[]>()
  for (const t of trials) {
    const list = byVideo.get(t.videoId)
    if (list) list.push(t)
    else byVideo.set(t.videoId, [t])
  }
  return [...byVideo.values()].map((list) => {
    const sorted = list.toSorted((a, b) => a.clipIndex - b.clipIndex)
    const count = (s: ModelTrialStatus) => sorted.filter((t) => t.status === s).length
    return {
      videoId: list[0].videoId,
      videoName: list[0].videoName,
      trials: sorted,
      done: count(ModelTrialStatus.SUCCEEDED),
      queued: count(ModelTrialStatus.QUEUED),
      running: count(ModelTrialStatus.RUNNING),
      failed: count(ModelTrialStatus.FAILED),
      framesDone: sorted.reduce((n, t) => n + t.framesDone, 0),
      framesTotal: sorted.reduce((n, t) => n + t.framesTotal, 0),
      score: scoreTrials(sorted),
    }
  })
}
