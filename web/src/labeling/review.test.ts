import { create } from '@bufbuild/protobuf'
import { describe, expect, it } from 'vitest'
import { AnnotationSchema, AnnotationStatus, type Annotation } from '@/gen/krill/v1/annotation_pb'
import { FrameSchema } from '@/gen/krill/v1/clip_pb'
import { reviewTracks } from './review'

const frames = [1n, 2n, 3n, 4n].map((id) => create(FrameSchema, { id }))

function box(frameId: bigint, x: number, width: number, status: AnnotationStatus): Annotation {
  return create(AnnotationSchema, {
    trackId: 9n,
    frameId,
    box: { x, y: 0.1, width, height: 0.2 },
    status,
  })
}

function byFrame(...boxes: Annotation[]) {
  return Object.fromEntries(boxes.map((a) => [String(a.frameId), { '9': a }]))
}

const { VERIFIED, PROPOSED } = AnnotationStatus

describe('reviewTracks', () => {
  it('flags a proposal that jumps away from the previous box', () => {
    const { drift } = reviewTracks(
      frames,
      byFrame(
        box(1n, 0.1, 0.2, VERIFIED),
        box(2n, 0.11, 0.2, PROPOSED),
        box(3n, 0.5, 0.2, PROPOSED),
        box(4n, 0.51, 0.2, PROPOSED),
      ),
    )
    expect([...drift.keys()]).toEqual(['3'])
  })

  it('flags a proposal that grows too much in place', () => {
    const { drift } = reviewTracks(
      frames,
      byFrame(box(1n, 0.1, 0.2, VERIFIED), box(2n, 0.1, 0.4, PROPOSED)),
    )
    expect(drift.get('2')?.has('9')).toBe(true)
  })

  it('does not flag boxes a labeler drew', () => {
    const { drift } = reviewTracks(
      frames,
      byFrame(box(1n, 0.1, 0.2, VERIFIED), box(2n, 0.6, 0.2, VERIFIED)),
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
})
