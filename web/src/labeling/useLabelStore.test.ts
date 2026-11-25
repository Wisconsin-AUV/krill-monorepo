import { create } from '@bufbuild/protobuf'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import {
  AnnotationSchema,
  AnnotationStatus,
  FrameStatus,
  SetBoxResponseSchema,
  type SetBoxResponse,
  type SetFrameStatusResponse,
} from '@/gen/krill/v1/annotation_pb'
import { FrameSchema, GetClipResponseSchema } from '@/gen/krill/v1/clip_pb'
import { LabelTypeSchema } from '@/gen/krill/v1/label_pb'
import { isLabeled } from '@/lib/keypoints'
import { keypointKeys, placeKeypoint } from './keypointEdit'
import { localBackend } from './localBackend'
import { useLabelStore, type AnnotationBackend } from './useLabelStore'

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

describe('setKeypoints', () => {
  it('keeps a keypoint placed while an earlier save is answered', async () => {
    const answers: (() => void)[] = []
    const setBox: AnnotationBackend['setBox'] = (req) =>
      new Promise<SetBoxResponse>((resolve) =>
        answers.push(() =>
          resolve(
            create(SetBoxResponseSchema, {
              annotation: { trackId: req.trackId, frameId: req.frameId, box: req.box },
            }),
          ),
        ),
      )
    const clip = create(GetClipResponseSchema, {
      frames: [create(FrameSchema, { id: 1n })],
      annotations: [{ trackId: 1n, frameId: 1n, box: { width: 0.5, height: 0.5 } }],
    })
    useLabelStore.getState().load(clip, { ...localBackend(), setBox })
    const type = create(LabelTypeSchema, {
      keypoints: ['a', 'b', 'c'].map((name) => ({ name })),
    })
    const place = () => placeKeypoint(type, 1n, 1n, { x: 0.2, y: 0.2 }, false, 1)

    place()
    place()
    await vi.waitFor(() => expect(answers).toHaveLength(1))
    answers[0]()
    await vi.waitFor(() => expect(answers).toHaveLength(2))
    place()
    answers[1]()
    await vi.waitFor(() => expect(answers).toHaveLength(3))
    answers[2]()
    await vi.waitFor(() => expect(useLabelStore.getState().pending).toBe(0))

    const kps = useLabelStore.getState().boxes['1']['1'].box?.keypoints ?? []
    expect(kps.map(isLabeled)).toEqual([true, true, true])
  })
})

// A selected box of a sheet type with keypoints 1 to 4 its corners and 5 its
// center.
function poseSheet() {
  const clip = create(GetClipResponseSchema, {
    frames: [create(FrameSchema, { id: 1n })],
    annotations: [{ trackId: 1n, frameId: 1n, box: { width: 0.5, height: 0.5 } }],
  })
  const setBox: AnnotationBackend['setBox'] = async (req) =>
    create(SetBoxResponseSchema, {
      annotation: { trackId: req.trackId, frameId: req.frameId, box: req.box },
    })
  useLabelStore.getState().load(clip, { ...localBackend(), setBox })
  useLabelStore.getState().select(1n)
  const type = create(LabelTypeSchema, {
    keypoints: ['tl', 'tr', 'br', 'bl', 'center'].map((name) => ({ name })),
    quads: [{ corners: [0, 1, 2, 3], center: 4 }],
  })
  const keys = keypointKeys(type, 1n, 1n)
  return {
    press: (key: string) => keys[key](new KeyboardEvent('keydown', { key })),
    place: (x: number, y: number) => placeKeypoint(type, 1n, 1n, { x, y }, false, 1),
    placed: async () => {
      await vi.waitFor(() => expect(useLabelStore.getState().pending).toBe(0))
      const kps = useLabelStore.getState().boxes['1']['1'].box?.keypoints ?? []
      return kps.map((k) => (isLabeled(k) ? [k.x, k.y] : null))
    },
  }
}

describe('keypointKeys', () => {
  it('places the keypoint picked by number, then the next unlabeled one after it', async () => {
    const { press, place, placed } = poseSheet()

    place(0.1, 0.1)
    press('3')
    place(0.4, 0.4)
    place(0.1, 0.4)
    press('5')
    press('9')
    place(0.4, 0.1)

    const kps = await placed()
    expect(kps.slice(0, 4)).toEqual([
      [0.1, 0.1],
      [0.4, 0.1],
      [0.4, 0.4],
      [0.1, 0.4],
    ])
    expect(kps[4]).not.toBeNull()
    expect(useLabelStore.getState().selectedTrackId).toBe(1n)
  })
})

describe('placeKeypoint', () => {
  it('places the unplaced corner nearest a click once two corners show where the rest are', async () => {
    const { place, placed } = poseSheet()

    place(0.2, 0.2)
    place(0.6, 0.2)
    // A wall hides the bottom right, so the labeler clicks the bottom left
    // while the bottom right is up next.
    place(0.25, 0.5)

    expect(await placed()).toEqual([[0.2, 0.2], [0.6, 0.2], null, [0.25, 0.5], null])
  })

  it('places a picked corner wherever the click is', async () => {
    const { press, place, placed } = poseSheet()

    place(0.2, 0.2)
    place(0.6, 0.2)
    press('3')
    place(0.25, 0.5)

    expect(await placed()).toEqual([[0.2, 0.2], [0.6, 0.2], [0.25, 0.5], null, null])
  })
})
