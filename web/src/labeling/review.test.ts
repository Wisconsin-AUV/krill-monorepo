import { create } from '@bufbuild/protobuf'
import { describe, expect, it } from 'vitest'
import {
  AnnotationSchema,
  AnnotationSource,
  AnnotationStatus,
  type Annotation,
} from '@/gen/krill/v1/annotation_pb'
import { FrameSchema } from '@/gen/krill/v1/clip_pb'
import { reviewTracks } from './review'

const frames = [1n, 2n, 3n, 4n].map((id) => create(FrameSchema, { id }))

function box(
  frameId: bigint,
  x: number,
  width: number,
  status: AnnotationStatus,
  drift = false,
  source: AnnotationSource = AnnotationSource.SAM,
): Annotation {
  return create(AnnotationSchema, {
    trackId: 9n,
    frameId,
    box: { x, y: 0.1, width, height: 0.2 },
    status,
    drift,
    source,
  })
}

function byFrame(...boxes: Annotation[]) {
  return Object.fromEntries(boxes.map((a) => [String(a.frameId), { '9': a }]))
}

const { VERIFIED, PROPOSED } = AnnotationStatus

describe('reviewTracks', () => {
  it('marks frames with proposals the server flagged', () => {
    const { drift } = reviewTracks(
      frames,
      byFrame(
        box(1n, 0.1, 0.2, VERIFIED),
        box(2n, 0.11, 0.2, PROPOSED),
        box(3n, 0.5, 0.2, PROPOSED, true),
        box(4n, 0.51, 0.2, PROPOSED),
      ),
    )
    expect([...drift.keys()]).toEqual(['3'])
    expect(drift.get('3')?.has('9')).toBe(true)
  })

  it('drops the flag once the box is verified locally', () => {
    const { drift } = reviewTracks(
      frames,
      byFrame(box(1n, 0.1, 0.2, VERIFIED), box(2n, 0.6, 0.2, VERIFIED, true)),
    )
    expect(drift.size).toBe(0)
  })

  it('leaves a track unconfirmed until its last box is verified', () => {
    const early = [box(1n, 0.1, 0.2, VERIFIED), box(2n, 0.1, 0.2, VERIFIED)]
    expect(
      reviewTracks(frames, byFrame(...early, box(3n, 0.1, 0.2, PROPOSED))).unconfirmed.has('9'),
    ).toBe(true)
    expect(
      reviewTracks(frames, byFrame(...early, box(3n, 0.1, 0.2, VERIFIED))).unconfirmed.has('9'),
    ).toBe(false)
  })

  it('never holds back a pre-label track for its last box', () => {
    const early = [box(1n, 0.1, 0.2, VERIFIED), box(2n, 0.1, 0.2, VERIFIED)]
    const last = box(3n, 0.1, 0.2, PROPOSED, false, AnnotationSource.YOLO)
    expect(reviewTracks(frames, byFrame(...early, last)).unconfirmed.has('9')).toBe(false)
  })
})
