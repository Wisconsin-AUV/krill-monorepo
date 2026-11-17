import type Konva from 'konva'
import { useMemo, useState, type ReactNode } from 'react'
import type { Annotation } from '@/gen/krill/v1/annotation_pb'
import type { LabelType } from '@/gen/krill/v1/label_pb'
import { hasKeypoints, padKeypoints, promptQuad } from '@/lib/keypoints'
import { useUIStore } from '@/store/useUIStore'
import { correctFrame } from '@/workspace/correctedFrames'
import { FrameCanvas } from '@/workspace/FrameCanvas'
import { loadFrame } from '@/workspace/imageCache'
import { useWorkspaceStore } from '@/workspace/useWorkspaceStore'
import { AnnotationLayer, type TrackInfo } from './AnnotationLayer'
import { activeKeypoint, placeKeypoint } from './keypointEdit'
import { Loupe } from './Loupe'
import type { TrackReview } from './review'
import { TrackingShimmer } from './TrackingShimmer'
import { idKey, useLabelStore, type BoxRect } from './useLabelStore'

const MIN_DRAG_PX = 4

function normalize(
  a: { x: number; y: number },
  b: { x: number; y: number },
  w: number,
  h: number,
): BoxRect {
  const x0 = Math.max(0, Math.min(a.x, b.x))
  const y0 = Math.max(0, Math.min(a.y, b.y))
  const x1 = Math.min(w, Math.max(a.x, b.x))
  const y1 = Math.min(h, Math.max(a.y, b.y))
  return { x: x0 / w, y: y0 / h, width: Math.max(x1 - x0, 0) / w, height: Math.max(y1 - y0, 0) / h }
}

export function LabelCanvas({
  tracks,
  types,
  drift,
  canTrack = true,
  overlay,
}: {
  tracks: Map<string, TrackInfo>
  types: LabelType[]
  drift: TrackReview['drift']
  canTrack?: boolean
  overlay?: ReactNode
}) {
  const frames = useWorkspaceStore((s) => s.frames)
  const index = useWorkspaceStore((s) => s.index)
  const view = useWorkspaceStore((s) => s.view)
  const imageSize = useWorkspaceStore((s) => s.imageSize)
  const stageSize = useWorkspaceStore((s) => s.stageSize)
  const boxes = useLabelStore((s) => s.boxes)
  const selectedTrackId = useLabelStore((s) => s.selectedTrackId)
  const activeTypeId = useLabelStore((s) => s.activeTypeId)
  const drawBox = useLabelStore((s) => s.drawBox)
  const trackObject = useLabelStore((s) => s.trackObject)
  const select = useLabelStore((s) => s.select)
  const tracking = useLabelStore((s) => s.tracking)
  const trackRuns = useLabelStore((s) => s.trackRuns)
  const hideBoxes = useLabelStore((s) => s.hideBoxes)
  const keypointFocus = useLabelStore((s) => s.keypointFocus)
  const corrected = useUIStore((s) => s.displayCorrection)
  const searching = useLabelStore((s) =>
    s.exemplarSearches.some((x) => x.frameId === frames[index]?.id),
  )
  const [draft, setDraft] = useState<BoxRect | null>(null)
  const [pointer, setPointer] = useState<{ x: number; y: number } | null>(null)
  const [keypointDrag, setKeypointDrag] = useState<{ x: number; y: number } | null>(null)

  const frame = frames[index]
  const frameEntry = useMemo(() => (frame ? loadFrame(frame.id, frame.url) : undefined), [frame])
  const fixedEntry = useMemo(
    () => (frame && frameEntry && corrected ? correctFrame(frame.id, frameEntry.ready) : undefined),
    [frame, frameEntry, corrected],
  )
  const toImage = (p: { x: number; y: number }) => ({
    x: (p.x - view.x) / view.scale,
    y: (p.y - view.y) / view.scale,
  })

  // The selected track's most recent box before this frame, drawn dashed so
  // the labeler can see where the object was when continuing the track. In a
  // gap between boxes it would sit where a just-deleted box was and make the
  // delete look like it did nothing.
  const ghost = useMemo((): Annotation | undefined => {
    if (selectedTrackId === null || !frame) return undefined
    const k = idKey(selectedTrackId)
    if (boxes[idKey(frame.id)]?.[k]) return undefined
    for (let i = index + 1; i < frames.length; i++) {
      if (boxes[idKey(frames[i].id)]?.[k]) return undefined
    }
    for (let i = index - 1; i >= 0; i--) {
      const a = boxes[idKey(frames[i].id)]?.[k]
      if (a) return a
    }
    return undefined
  }, [boxes, frame, frames, index, selectedTrackId])

  const selectedInfo = selectedTrackId !== null ? tracks.get(idKey(selectedTrackId)) : undefined
  const selectedHere =
    frame && selectedTrackId !== null ? boxes[idKey(frame.id)]?.[idKey(selectedTrackId)] : undefined
  const poseType =
    selectedHere && !hideBoxes && hasKeypoints(selectedInfo?.type) ? selectedInfo.type : undefined
  const poseKeypoints = poseType
    ? padKeypoints(selectedHere?.box?.keypoints ?? [], poseType.keypoints.length)
    : []
  const active =
    poseType && frame && selectedTrackId !== null
      ? activeKeypoint(keypointFocus, poseKeypoints, poseType, selectedTrackId, frame.id)
      : null

  const toNorm = (p: { x: number; y: number }) => ({
    x: p.x / imageSize.width,
    y: p.y / imageSize.height,
  })

  // p is in image pixels.
  function place(p: { x: number; y: number }, hidden: boolean): boolean {
    if (!poseType || !frame || selectedTrackId === null) return false
    return placeKeypoint(poseType, selectedTrackId, frame.id, toNorm(p), hidden)
  }

  function onSelectedClick(p: { x: number; y: number }, evt: MouseEvent) {
    if (!frame || selectedTrackId === null || evt.altKey) return
    if (!evt.shiftKey) {
      place(p, false)
      return
    }
    const type = selectedInfo?.type
    if (!canTrack || !type || type.quads.length === 0) return
    const kps = padKeypoints(selectedHere?.box?.keypoints ?? [], type.keypoints.length)
    const point = toNorm(p)
    void trackObject(
      frame.id,
      { quad: { quad: promptQuad(type, kps, active, point), point } },
      selectedTrackId,
    )
  }

  const continuing =
    selectedTrackId !== null && frame && !boxes[idKey(frame.id)]?.[idKey(selectedTrackId)]
  const drawType = continuing
    ? tracks.get(idKey(selectedTrackId))?.type
    : types.find((t) => t.id === activeTypeId)

  function onStageMouseDown(e: Konva.KonvaEventObject<MouseEvent>) {
    const stage = e.target.getStage()
    if (!stage || e.target !== stage || e.evt.button !== 0 || !frame) return
    const rect = stage.container().getBoundingClientRect()
    const start = toImage({ x: e.evt.clientX - rect.left, y: e.evt.clientY - rect.top })
    const frameId = frame.id
    const track = canTrack && e.evt.shiftKey
    let moved = false

    const onMove = (ev: MouseEvent) => {
      const p = toImage({ x: ev.clientX - rect.left, y: ev.clientY - rect.top })
      const dx = Math.abs(p.x - start.x) * view.scale
      const dy = Math.abs(p.y - start.y) * view.scale
      if (dx > MIN_DRAG_PX || dy > MIN_DRAG_PX) moved = true
      if (moved) setDraft(normalize(start, p, imageSize.width, imageSize.height))
    }
    const onUp = (ev: MouseEvent) => {
      window.removeEventListener('mousemove', onMove)
      window.removeEventListener('mouseup', onUp)
      setDraft(null)
      if (!moved) {
        if (track) {
          const point = { x: start.x / imageSize.width, y: start.y / imageSize.height }
          if (point.x >= 0 && point.x <= 1 && point.y >= 0 && point.y <= 1)
            void trackObject(frameId, { point })
        } else if (!place(start, false)) {
          select(null)
        }
        return
      }
      const p = toImage({ x: ev.clientX - rect.left, y: ev.clientY - rect.top })
      const box = normalize(start, p, imageSize.width, imageSize.height)
      if (box.width <= 0 || box.height <= 0) return
      if (track) void trackObject(frameId, { box })
      else void drawBox(frameId, box)
    }
    window.addEventListener('mousemove', onMove)
    window.addEventListener('mouseup', onUp)
  }

  const newType = types.find((t) => t.id === activeTypeId)
  const hint = !drawType
    ? 'Pick a label type to draw'
    : continuing
      ? `Drag to continue ${drawType.name} #${selectedInfo?.number ?? ''}`
      : `Drag to draw a new ${drawType.name}`
  const trackHint = canTrack && newType ? ` · Shift+click to track a new ${newType.name}` : ''
  const insideHint = drawType ? ' · Hold ⌘/Ctrl to draw inside a box' : ''
  const loupeAt = keypointDrag ?? (active !== null && !draft ? pointer : null)
  const loupeImage = corrected ? fixedEntry?.image : frameEntry?.loaded ? frameEntry.img : undefined
  const runsHere = frame
    ? trackRuns.filter(
        (r) => r.frameId === frame.id && (r.trackId === undefined || tracking[idKey(r.trackId)]),
      )
    : []

  return (
    <>
      <FrameCanvas
        onStageMouseDown={onStageMouseDown}
        onPointerMove={(p) => setPointer(p ? toImage(p) : null)}
        onAltClick={(p) => place(toImage(p), true)}
        cursor={drawType ? 'crosshair' : 'default'}
        corrected={corrected}
      >
        {frame && (
          <AnnotationLayer
            frameId={frame.id}
            ghost={ghost}
            imageSize={imageSize}
            scale={view.scale}
            tracks={tracks}
            drifted={drift.get(idKey(frame.id))}
            draft={draft}
            draftColor={drawType?.color ?? '#a1a1aa'}
            pointer={drawType ? pointer : null}
            activeKeypoint={active}
            onKeypointDrag={setKeypointDrag}
            onSelectedClick={onSelectedClick}
          />
        )}
        {overlay}
      </FrameCanvas>
      {runsHere.map((r, i) => (
        <TrackingShimmer
          key={i}
          prompt={r.prompt}
          box={
            r.trackId === undefined ? undefined : boxes[idKey(r.frameId)]?.[idKey(r.trackId)]?.box
          }
          view={view}
          imageSize={imageSize}
        />
      ))}
      {loupeAt && loupeImage && (
        <Loupe image={loupeImage} at={loupeAt} view={view} stageSize={stageSize} />
      )}
      {poseType && active !== null && pointer && !draft && !keypointDrag && (
        <div
          className="pointer-events-none absolute rounded-sm bg-zinc-900/80 px-1 text-xs/5 text-zinc-100"
          style={{
            left: view.x + pointer.x * view.scale + 12,
            top: view.y + pointer.y * view.scale + 12,
          }}
        >
          {active + 1} {poseType.keypoints[active]?.name}
        </div>
      )}
      {searching && (
        <TrackingShimmer
          prompt={{ box: { x: 0, y: 0, width: 1, height: 1 } }}
          view={view}
          imageSize={imageSize}
        />
      )}
      <div className="pointer-events-none absolute top-3 left-3 flex items-center gap-2 rounded-md bg-zinc-900/80 px-2.5 py-1 text-xs/5 text-zinc-300 ring-1 ring-white/10 backdrop-blur">
        {drawType && (
          <span className="size-2 rounded-full" style={{ backgroundColor: drawType.color }} />
        )}
        {hint}
        {trackHint}
        {insideHint}
      </div>
    </>
  )
}
