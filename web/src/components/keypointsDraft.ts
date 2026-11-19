import type { LabelType } from '@/gen/krill/v1/label_pb'

export interface KeypointDraft {
  id: number
  name: string
  mirror: number | null
  // Index in the saved type, which boxes' labeled keypoints are stored by.
  // Null for a keypoint the type doesn't have yet.
  origin: number | null
}

export interface QuadDraft {
  id: number
  corners: (number | null)[]
  reference: boolean
  center: number | null
}

export interface EdgeDraft {
  id: number
  from: number | null
  to: number | null
}

// Quads, edges, and mirrors point at keypoints by draft id rather than index
// so they follow a keypoint when the list is reordered.
export interface KeypointsDraft {
  keypoints: KeypointDraft[]
  quads: QuadDraft[]
  edges: EdgeDraft[]
}

let nextId = 0

export function draftId() {
  return nextId++
}

export function toKeypointsDraft(type?: LabelType): KeypointsDraft {
  const ids = (type?.keypoints ?? []).map(() => nextId++)
  const id = (i: number | undefined) => (i === undefined ? null : (ids[i] ?? null))
  return {
    keypoints: (type?.keypoints ?? []).map((k, i) => ({
      id: ids[i] ?? nextId++,
      name: k.name,
      mirror: id(k.mirror),
      origin: i,
    })),
    quads: (type?.quads ?? []).map((q) => ({
      id: nextId++,
      corners: q.corners.map(id),
      reference: q.reference,
      center: id(q.center),
    })),
    edges: (type?.edges ?? []).map((e) => ({ id: nextId++, from: id(e.from), to: id(e.to) })),
  }
}

export function keypointsComplete(d: KeypointsDraft): boolean {
  return (
    d.quads.every((q) => q.corners.every((c) => c !== null)) &&
    d.edges.every((e) => e.from !== null && e.to !== null)
  )
}

export function keypointsBody(d: KeypointsDraft) {
  const index = new Map(d.keypoints.map((k, i) => [k.id, i]))
  const at = (id: number | null) => (id === null ? undefined : index.get(id))
  return {
    keypoints: d.keypoints.map((k) => ({
      name: k.name.trim().toLowerCase(),
      mirror: at(k.mirror),
    })),
    quads: d.quads.map((q) => ({
      corners: q.corners.map((c) => at(c) ?? 0),
      reference: q.reference,
      center: at(q.center),
    })),
    edges: d.edges.map((e) => ({ from: at(e.from) ?? 0, to: at(e.to) ?? 0 })),
  }
}

export function keypointOrigins(d: KeypointsDraft): number[] {
  return d.keypoints.map((k) => k.origin ?? -1)
}

// Keypoints of the saved type the draft no longer has, which saving clears
// from every box.
export function removedKeypoints(type: LabelType, d: KeypointsDraft): string[] {
  const kept = new Set(d.keypoints.map((k) => k.origin))
  return type.keypoints.filter((_, i) => !kept.has(i)).map((k) => k.name)
}
