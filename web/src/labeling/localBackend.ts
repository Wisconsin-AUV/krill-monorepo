import { create } from '@bufbuild/protobuf'
import {
  AnnotationSchema,
  AnnotationSource,
  AnnotationStatus,
  CopyBoxesResponseSchema,
  CreateTrackResponseSchema,
  DeleteBoxResponseSchema,
  DeleteTrackResponseSchema,
  FrameStatus,
  SetBoxResponseSchema,
  SetFrameStatusResponseSchema,
  TrackSchema,
  UpdateTrackResponseSchema,
  type Annotation,
  type Box,
  type Track,
} from '@/gen/krill/v1/annotation_pb'
import type { AnnotationBackend } from './useLabelStore'

type BoxInit = Partial<Pick<Box, 'x' | 'y' | 'width' | 'height'>> | undefined

// localBackend keeps edits in memory, for labeling a frame without saving it.
export function localBackend(): AnnotationBackend {
  let nextId = 1n
  const tracks = new Map<bigint, Track>()
  const boxes = new Map<string, Annotation>()
  const boxKey = (trackId: bigint, frameId: bigint) => `${trackId}:${frameId}`

  function putBox(trackId: bigint, frameId: bigint, box: BoxInit): Annotation {
    const a = create(AnnotationSchema, {
      id: nextId++,
      trackId,
      frameId,
      box,
      source: AnnotationSource.HUMAN,
      status: AnnotationStatus.VERIFIED,
    })
    boxes.set(boxKey(trackId, frameId), a)
    return a
  }

  function track(id: bigint | undefined): Track {
    const t = tracks.get(id ?? 0n)
    if (!t) throw new Error('Track not found')
    return t
  }

  return {
    createTrack: async (req) => {
      const t = create(TrackSchema, {
        id: nextId++,
        clipId: req.clipId,
        labelTypeId: req.labelTypeId,
        attributes: req.attributes,
      })
      tracks.set(t.id, t)
      return create(CreateTrackResponseSchema, {
        track: t,
        annotation: putBox(t.id, req.frameId ?? 0n, req.box),
      })
    },
    updateTrack: async (req) => {
      const prev = track(req.id)
      const typeChanged = req.labelTypeId !== undefined && req.labelTypeId !== prev.labelTypeId
      const t = create(TrackSchema, {
        ...prev,
        labelTypeId: req.labelTypeId ?? prev.labelTypeId,
        attributes: req.setAttributes ? (req.attributes ?? {}) : typeChanged ? {} : prev.attributes,
      })
      tracks.set(t.id, t)
      return create(UpdateTrackResponseSchema, { track: t })
    },
    deleteTrack: async (req) => {
      tracks.delete(req.id ?? 0n)
      for (const [k, a] of boxes) if (a.trackId === req.id) boxes.delete(k)
      return create(DeleteTrackResponseSchema)
    },
    setBox: async (req) => {
      const t = track(req.trackId)
      return create(SetBoxResponseSchema, { annotation: putBox(t.id, req.frameId ?? 0n, req.box) })
    },
    deleteBox: async (req) => {
      const t = track(req.trackId)
      boxes.delete(boxKey(t.id, req.frameId ?? 0n))
      const empty = ![...boxes.values()].some((a) => a.trackId === t.id)
      if (empty) tracks.delete(t.id)
      return create(DeleteBoxResponseSchema, { trackDeleted: empty })
    },
    copyBoxes: async () => create(CopyBoxesResponseSchema),
    setFrameStatus: async (req) =>
      create(SetFrameStatusResponseSchema, { status: req.status ?? FrameStatus.UNLABELED }),
    trackObject: async () => {
      throw new Error('Tracking is off for gold checks.')
    },
  }
}
