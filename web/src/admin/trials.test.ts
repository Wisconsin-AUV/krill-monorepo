import { create } from '@bufbuild/protobuf'
import { describe, expect, it } from 'vitest'
import { ModelTrialSchema, ModelTrialStatus } from '@/gen/krill/v1/pipeline_pb'
import { groupTrials } from './trials'

function trial(
  id: bigint,
  clipId: bigint,
  status: ModelTrialStatus,
  counts?: { matched: number; missed: number; extra: number },
) {
  return create(ModelTrialSchema, {
    id,
    clipId,
    clipIndex: Number(clipId),
    videoId: 1n,
    videoName: 'v',
    status,
    framesTotal: 10,
    framesDone: status === ModelTrialStatus.SUCCEEDED ? 10 : 4,
    summary: counts && { all: counts },
  })
}

describe('groupTrials', () => {
  it('pools the finished trials of a video in clip order', () => {
    const [g] = groupTrials([
      trial(4n, 2n, ModelTrialStatus.SUCCEEDED, { matched: 6, missed: 0, extra: 2 }),
      trial(3n, 3n, ModelTrialStatus.RUNNING),
      trial(2n, 1n, ModelTrialStatus.SUCCEEDED, { matched: 2, missed: 4, extra: 0 }),
    ])
    expect(g.trials.map((t) => t.id)).toEqual([2n, 4n, 3n])
    expect([g.done, g.running, g.framesDone, g.framesTotal]).toEqual([2, 1, 24, 30])
    expect(g.score).toEqual({ matched: 8, missed: 4, extra: 2, precision: 0.8, recall: 8 / 12 })
  })

  it('leaves ratios unset when nothing is scored', () => {
    expect(groupTrials([trial(1n, 1n, ModelTrialStatus.FAILED)])[0].score).toBeUndefined()
    expect(
      groupTrials([
        trial(1n, 1n, ModelTrialStatus.SUCCEEDED, { matched: 0, missed: 0, extra: 0 }),
      ])[0].score,
    ).toEqual({ matched: 0, missed: 0, extra: 0, precision: undefined, recall: undefined })
  })
})
