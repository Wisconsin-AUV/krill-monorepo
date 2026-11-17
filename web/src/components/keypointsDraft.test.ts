import { create } from '@bufbuild/protobuf'
import { describe, expect, it } from 'vitest'
import { LabelTypeSchema } from '@/gen/krill/v1/label_pb'
import { keypointsBody, toKeypointsDraft } from './keypointsDraft'

const type = create(LabelTypeSchema, {
  keypoints: [
    { name: 'left_eye', mirror: 1 },
    { name: 'right_eye', mirror: 0 },
    { name: 'paper_tl' },
    { name: 'paper_tr' },
    { name: 'paper_br' },
    { name: 'paper_bl' },
    { name: 'paper_center' },
  ],
  quads: [{ corners: [2, 3, 4, 5], reference: true, center: 6 }],
  edges: [{ from: 0, to: 2 }],
})

describe('keypointsBody', () => {
  it('round trips a saved type', () => {
    const body = keypointsBody(toKeypointsDraft(type))
    expect(body.keypoints.map((k) => k.mirror)).toEqual([
      1,
      0,
      undefined,
      undefined,
      undefined,
      undefined,
      undefined,
    ])
    expect(body.quads).toEqual([{ corners: [2, 3, 4, 5], reference: true, center: 6 }])
    expect(body.edges).toEqual([{ from: 0, to: 2 }])
  })

  it('follows keypoints to their new indexes when reordered', () => {
    const draft = toKeypointsDraft(type)
    const body = keypointsBody({ ...draft, keypoints: [...draft.keypoints].reverse() })
    expect(body.keypoints.map((k) => k.name)[0]).toBe('paper_center')
    expect(body.keypoints[5]?.mirror).toBe(6)
    expect(body.keypoints[6]?.mirror).toBe(5)
    expect(body.quads).toEqual([{ corners: [4, 3, 2, 1], reference: true, center: 0 }])
    expect(body.edges).toEqual([{ from: 6, to: 4 }])
  })
})
