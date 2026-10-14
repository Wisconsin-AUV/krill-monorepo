import { create } from '@bufbuild/protobuf'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import {
  AnnotationSchema,
  AnnotationStatus,
  FrameStatus,
  type SetFrameStatusResponse,
} from '@/gen/krill/v1/annotation_pb'
import { FrameSchema, GetClipResponseSchema } from '@/gen/krill/v1/clip_pb'
import { localBackend } from './localBackend'
import { useLabelStore } from './useLabelStore'

const { PROPOSED, VERIFIED } = AnnotationStatus

let rejectSave: ((err: Error) => void) | undefined

// Saves run one at a time, so the save has to be started before it can fail.
async function failSave() {
  await vi.waitFor(() => expect(rejectSave).toBeDefined())
  rejectSave?.(new Error('offline'))
}

beforeEach(() => {
  rejectSave = undefined
  const backend = {
    ...localBackend(),
    setFrameStatus: () =>
      new Promise<SetFrameStatusResponse>((_, reject) => {
        rejectSave = reject
      }),
  }
  const clip = create(GetClipResponseSchema, {
    frames: [create(FrameSchema, { id: 1n, status: FrameStatus.UNLABELED })],
    annotations: [1n, 2n].map((trackId) =>
      create(AnnotationSchema, { trackId, frameId: 1n, status: PROPOSED }),
    ),
  })
  useLabelStore.getState().load(clip, backend)
})

const status = () => useLabelStore.getState().frameStatus['1']
const boxStatus = (track: string) => useLabelStore.getState().boxes['1'][track].status

describe('setFrameStatus', () => {
  it('verifies the frame before the server answers', async () => {
    const saved = useLabelStore.getState().setFrameStatus(1n, FrameStatus.LABELED)
    expect(status()).toBe(FrameStatus.LABELED)
    expect(boxStatus('1')).toBe(VERIFIED)
    expect(useLabelStore.getState().undoStack).toHaveLength(1)
    await failSave()
    await saved
  })

  it('rolls back a failed save without undoing edits made while it was in flight', async () => {
    const saved = useLabelStore.getState().setFrameStatus(1n, FrameStatus.LABELED)
    useLabelStore.setState((s) => ({
      boxes: { '1': { ...s.boxes['1'], '2': { ...s.boxes['1']['2'] } } },
    }))
    await failSave()

    expect(await saved).toBe(false)
    expect(status()).toBe(FrameStatus.UNLABELED)
    expect(boxStatus('1')).toBe(PROPOSED)
    expect(boxStatus('2')).toBe(VERIFIED)
    expect(useLabelStore.getState().undoStack).toHaveLength(0)
  })
})
