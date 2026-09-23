import { create } from 'zustand'
import { FrameStatus, type Annotation, type Box, type Track } from '@/gen/krill/v1/annotation_pb'
import type { GetClipResponse } from '@/gen/krill/v1/clip_pb'
import { annotationClient } from '@/lib/clients'
import { flash } from '@/lib/flash'

export type BoxRect = Pick<Box, 'x' | 'y' | 'width' | 'height'>

type Key = string
const key = (id: bigint): Key => String(id)

interface UndoEntry {
  label: string
  run: () => Promise<void>
}

interface LabelState {
  clipId: bigint | null
  tracks: Record<Key, Track>
  // frame id -> track id -> box
  boxes: Record<Key, Record<Key, Annotation>>
  frameStatus: Record<Key, FrameStatus>
  selectedTrackId: bigint | null
  activeTypeId: bigint | null
  hideBoxes: boolean
  pending: number
  undoStack: UndoEntry[]

  load: (clip: GetClipResponse) => void
  select: (trackId: bigint | null) => void
  setActiveType: (id: bigint | null) => void
  toggleHidden: () => void

  drawBox: (frameId: bigint, box: BoxRect) => Promise<void>
  moveBox: (trackId: bigint, frameId: bigint, box: BoxRect) => Promise<void>
  deleteBox: (trackId: bigint, frameId: bigint) => Promise<void>
  deleteTrack: (trackId: bigint) => Promise<void>
  updateTrack: (
    trackId: bigint,
    patch: { labelTypeId?: bigint; attributes?: Record<string, string> },
  ) => Promise<void>
  copyBoxes: (fromFrameId: bigint, toFrameId: bigint) => Promise<number>
  setFrameStatus: (frameId: bigint, status: FrameStatus) => Promise<boolean>
  undo: () => Promise<void>
}

const MAX_UNDO = 100

// Tracks deleted and recreated by undo get new ids. Older undo entries still
// hold the original id, so every id is resolved through this map first.
const remap = new Map<Key, bigint>()
function resolve(id: bigint): bigint {
  let current = id
  for (let next = remap.get(key(current)); next !== undefined; next = remap.get(key(current))) {
    current = next
  }
  return current
}

// Edits run one at a time so a quick drag followed by a delete can never
// reach the server out of order.
let queue: Promise<unknown> = Promise.resolve()
function enqueue<T>(fn: () => Promise<T>): Promise<T> {
  const run = queue.then(fn, fn)
  queue = run.catch(() => undefined)
  return run
}

function rect(box: Box | undefined): BoxRect {
  return { x: box?.x ?? 0, y: box?.y ?? 0, width: box?.width ?? 0, height: box?.height ?? 0 }
}

export const useLabelStore = create<LabelState>((set, get) => {
  function putBox(a: Annotation) {
    set((s) => ({
      boxes: {
        ...s.boxes,
        [key(a.frameId)]: { ...s.boxes[key(a.frameId)], [key(a.trackId)]: a },
      },
    }))
  }

  function removeBox(trackId: bigint, frameId: bigint) {
    set((s) => {
      const frame = { ...s.boxes[key(frameId)] }
      delete frame[key(trackId)]
      return { boxes: { ...s.boxes, [key(frameId)]: frame } }
    })
  }

  function removeTrack(trackId: bigint) {
    set((s) => {
      const tracks = { ...s.tracks }
      delete tracks[key(trackId)]
      const boxes: LabelState['boxes'] = {}
      for (const [f, byTrack] of Object.entries(s.boxes)) {
        if (!byTrack[key(trackId)]) {
          boxes[f] = byTrack
          continue
        }
        const copy = { ...byTrack }
        delete copy[key(trackId)]
        boxes[f] = copy
      }
      return {
        tracks,
        boxes,
        selectedTrackId: s.selectedTrackId === trackId ? null : s.selectedTrackId,
      }
    })
  }

  // A frame marked empty that gains a box goes back to unlabeled, matching
  // what the server does.
  function clearEmpty(frameId: bigint) {
    if (get().frameStatus[key(frameId)] === FrameStatus.EMPTY) {
      set((s) => ({ frameStatus: { ...s.frameStatus, [key(frameId)]: FrameStatus.UNLABELED } }))
    }
  }

  function pushUndo(entry: UndoEntry) {
    set((s) => ({ undoStack: [...s.undoStack.slice(-MAX_UNDO + 1), entry] }))
  }

  async function withPending<T>(title: string, fn: () => Promise<T>): Promise<T | undefined> {
    set((s) => ({ pending: s.pending + 1 }))
    try {
      return await enqueue(fn)
    } catch (err) {
      flash.error(title, err)
      return undefined
    } finally {
      set((s) => ({ pending: s.pending - 1 }))
    }
  }

  async function recreateTrack(track: Track, boxes: Annotation[]): Promise<bigint | undefined> {
    const [first, ...rest] = boxes
    if (!first) return undefined
    const created = await annotationClient.createTrack({
      clipId: track.clipId,
      labelTypeId: track.labelTypeId,
      attributes: track.attributes,
      frameId: first.frameId,
      box: rect(first.box),
    })
    if (!created.track || !created.annotation) return undefined
    const newTrack = created.track
    remap.set(key(track.id), newTrack.id)
    set((s) => ({ tracks: { ...s.tracks, [key(newTrack.id)]: newTrack } }))
    putBox(created.annotation)
    clearEmpty(first.frameId)
    for (const b of rest) {
      const res = await annotationClient.setBox({
        trackId: newTrack.id,
        frameId: b.frameId,
        box: rect(b.box),
      })
      if (res.annotation) putBox(res.annotation)
      clearEmpty(b.frameId)
    }
    return newTrack.id
  }

  return {
    clipId: null,
    tracks: {},
    boxes: {},
    frameStatus: {},
    selectedTrackId: null,
    activeTypeId: null,
    hideBoxes: false,
    pending: 0,
    undoStack: [],

    load: (clip) => {
      const tracks: LabelState['tracks'] = {}
      for (const t of clip.tracks) tracks[key(t.id)] = t
      const boxes: LabelState['boxes'] = {}
      for (const a of clip.annotations) {
        const f = key(a.frameId)
        boxes[f] = { ...boxes[f], [key(a.trackId)]: a }
      }
      const frameStatus: LabelState['frameStatus'] = {}
      for (const f of clip.frames) frameStatus[key(f.id)] = f.status
      remap.clear()
      set({
        clipId: clip.clip?.id ?? null,
        tracks,
        boxes,
        frameStatus,
        selectedTrackId: null,
        undoStack: [],
      })
    },

    select: (trackId) => set({ selectedTrackId: trackId }),
    // Picking a type means the next box is a new object, so drop the
    // selection rather than continuing whatever track was selected.
    setActiveType: (id) => set({ activeTypeId: id, selectedTrackId: null }),
    toggleHidden: () => set((s) => ({ hideBoxes: !s.hideBoxes })),

    drawBox: async (frameId, box) => {
      const { selectedTrackId, boxes, activeTypeId, clipId } = get()
      const continuing = selectedTrackId !== null && !boxes[key(frameId)]?.[key(selectedTrackId)]
      if (continuing) {
        await get().moveBox(selectedTrackId, frameId, box)
        return
      }
      if (activeTypeId === null || clipId === null) {
        flash.error('Pick a label type first', 'Press 1 to 9 or click a type on the left.')
        return
      }
      await withPending('Could not create box', async () => {
        const res = await annotationClient.createTrack({
          clipId,
          labelTypeId: activeTypeId,
          frameId,
          box,
        })
        const track = res.track
        if (!track || !res.annotation) return
        set((s) => ({ tracks: { ...s.tracks, [key(track.id)]: track }, selectedTrackId: track.id }))
        putBox(res.annotation)
        clearEmpty(frameId)
        pushUndo({
          label: 'Undo new box',
          run: () =>
            enqueue(async () => {
              const id = resolve(track.id)
              await annotationClient.deleteTrack({ id })
              removeTrack(id)
            }),
        })
      })
    },

    moveBox: async (trackIdIn, frameId, box) => {
      const trackId = resolve(trackIdIn)
      const prev = get().boxes[key(frameId)]?.[key(trackId)]
      const optimistic = {
        ...(prev ?? { id: 0n, trackId, frameId }),
        box: { ...prev?.box, ...box },
      } as Annotation
      putBox(optimistic)
      await withPending('Could not save box', async () => {
        try {
          const res = await annotationClient.setBox({ trackId, frameId, box })
          if (res.annotation) putBox(res.annotation)
          clearEmpty(frameId)
        } catch (err) {
          if (prev) putBox(prev)
          else removeBox(trackId, frameId)
          throw err
        }
        pushUndo({
          label: prev ? 'Undo move' : 'Undo new box',
          run: () =>
            enqueue(async () => {
              const id = resolve(trackId)
              if (prev) {
                const res = await annotationClient.setBox({
                  trackId: id,
                  frameId,
                  box: rect(prev.box),
                })
                if (res.annotation) putBox(res.annotation)
              } else {
                const res = await annotationClient.deleteBox({ trackId: id, frameId })
                if (res.trackDeleted) removeTrack(id)
                else removeBox(id, frameId)
              }
            }),
        })
      })
    },

    deleteBox: async (trackIdIn, frameId) => {
      const trackId = resolve(trackIdIn)
      const prev = get().boxes[key(frameId)]?.[key(trackId)]
      const track = get().tracks[key(trackId)]
      if (!prev || !track) return
      await withPending('Could not delete box', async () => {
        const res = await annotationClient.deleteBox({ trackId, frameId })
        if (res.trackDeleted) removeTrack(trackId)
        else removeBox(trackId, frameId)
        pushUndo({
          label: 'Undo delete',
          run: () =>
            enqueue(async () => {
              if (res.trackDeleted) {
                await recreateTrack(track, [prev])
                return
              }
              const id = resolve(trackId)
              const restored = await annotationClient.setBox({
                trackId: id,
                frameId,
                box: rect(prev.box),
              })
              if (restored.annotation) putBox(restored.annotation)
            }),
        })
      })
    },

    deleteTrack: async (trackIdIn) => {
      const trackId = resolve(trackIdIn)
      const track = get().tracks[key(trackId)]
      if (!track) return
      const boxes = Object.values(get().boxes)
        .map((byTrack) => byTrack[key(trackId)])
        .filter((a): a is Annotation => !!a)
      await withPending('Could not delete track', async () => {
        await annotationClient.deleteTrack({ id: trackId })
        removeTrack(trackId)
        pushUndo({
          label: 'Undo delete track',
          run: () =>
            enqueue(async () => {
              await recreateTrack(track, boxes)
            }),
        })
      })
    },

    updateTrack: async (trackIdIn, patch) => {
      const trackId = resolve(trackIdIn)
      const prev = get().tracks[key(trackId)]
      if (!prev) return
      await withPending('Could not update track', async () => {
        const res = await annotationClient.updateTrack({
          id: trackId,
          labelTypeId: patch.labelTypeId,
          attributes: patch.attributes ?? {},
          setAttributes: patch.attributes !== undefined,
        })
        const track = res.track
        if (!track) return
        set((s) => ({ tracks: { ...s.tracks, [key(track.id)]: track } }))
        pushUndo({
          label: 'Undo track change',
          run: () =>
            enqueue(async () => {
              const restored = await annotationClient.updateTrack({
                id: resolve(trackId),
                labelTypeId: prev.labelTypeId,
                attributes: prev.attributes,
                setAttributes: true,
              })
              const t = restored.track
              if (t) set((s) => ({ tracks: { ...s.tracks, [key(t.id)]: t } }))
            }),
        })
      })
    },

    copyBoxes: async (fromFrameId, toFrameId) => {
      const copied = await withPending('Could not copy boxes', async () => {
        const res = await annotationClient.copyBoxes({ fromFrameId, toFrameId })
        res.annotations.forEach(putBox)
        if (res.annotations.length > 0) clearEmpty(toFrameId)
        const made = res.annotations.map((a) => a.trackId)
        if (made.length > 0) {
          pushUndo({
            label: 'Undo copy',
            run: () =>
              enqueue(async () => {
                for (const t of made) {
                  const id = resolve(t)
                  const r = await annotationClient.deleteBox({ trackId: id, frameId: toFrameId })
                  if (r.trackDeleted) removeTrack(id)
                  else removeBox(id, toFrameId)
                }
              }),
          })
        }
        return made.length
      })
      return copied ?? 0
    },

    setFrameStatus: async (frameId, status) => {
      const prev = get().frameStatus[key(frameId)] ?? FrameStatus.UNLABELED
      const ok = await withPending('Could not update frame', async () => {
        const res = await annotationClient.setFrameStatus({ frameId, status })
        set((s) => ({ frameStatus: { ...s.frameStatus, [key(frameId)]: res.status } }))
        if (prev !== res.status) {
          pushUndo({
            label: 'Undo frame status',
            run: () =>
              enqueue(async () => {
                const r = await annotationClient.setFrameStatus({ frameId, status: prev })
                set((s) => ({ frameStatus: { ...s.frameStatus, [key(frameId)]: r.status } }))
              }),
          })
        }
        return true
      })
      return ok ?? false
    },

    undo: async () => {
      const entry = get().undoStack.at(-1)
      if (!entry) return
      set((s) => ({ undoStack: s.undoStack.slice(0, -1) }))
      set((s) => ({ pending: s.pending + 1 }))
      try {
        await entry.run()
      } catch (err) {
        flash.error('Could not undo', err)
      } finally {
        set((s) => ({ pending: s.pending - 1 }))
      }
    },
  }
})

export { key as idKey }
