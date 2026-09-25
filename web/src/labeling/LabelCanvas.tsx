import type Konva from 'konva'
import { useMemo, useState } from 'react'
import type { Annotation } from '@/gen/krill/v1/annotation_pb'
import type { LabelType } from '@/gen/krill/v1/label_pb'
import { FrameCanvas } from '@/workspace/FrameCanvas'
import { useWorkspaceStore } from '@/workspace/useWorkspaceStore'
import { AnnotationLayer, type TrackInfo } from './AnnotationLayer'
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
}: {
  tracks: Map<string, TrackInfo>
  types: LabelType[]
}) {
  const frames = useWorkspaceStore((s) => s.frames)
  const index = useWorkspaceStore((s) => s.index)
  const view = useWorkspaceStore((s) => s.view)
  const imageSize = useWorkspaceStore((s) => s.imageSize)
  const boxes = useLabelStore((s) => s.boxes)
  const selectedTrackId = useLabelStore((s) => s.selectedTrackId)
  const activeTypeId = useLabelStore((s) => s.activeTypeId)
  const drawBox = useLabelStore((s) => s.drawBox)
  const trackObject = useLabelStore((s) => s.trackObject)
  const select = useLabelStore((s) => s.select)
  const tracking = useLabelStore((s) => s.tracking)
  const trackRuns = useLabelStore((s) => s.trackRuns)
  const [draft, setDraft] = useState<BoxRect | null>(null)
  const [pointer, setPointer] = useState<{ x: number; y: number } | null>(null)

  const frame = frames[index]
  const toImage = (p: { x: number; y: number }) => ({
    x: (p.x - view.x) / view.scale,
    y: (p.y - view.y) / view.scale,
  })

  // The selected track's most recent box before this frame, drawn dashed so
  // the labeler can see where the object was when continuing the track.
  const ghost = useMemo((): Annotation | undefined => {
    if (selectedTrackId === null || !frame) return undefined
    const k = idKey(selectedTrackId)
    if (boxes[idKey(frame.id)]?.[k]) return undefined
    for (let i = index - 1; i >= 0; i--) {
      const a = boxes[idKey(frames[i].id)]?.[k]
      if (a) return a
    }
    return undefined
  }, [boxes, frame, frames, index, selectedTrackId])

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
    const track = e.evt.shiftKey
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
        } else {
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

  const selectedInfo = selectedTrackId !== null ? tracks.get(idKey(selectedTrackId)) : undefined
  const newType = types.find((t) => t.id === activeTypeId)
  const hint = !drawType
    ? 'Pick a label type to draw'
    : continuing
      ? `Drag to continue ${drawType.name} #${selectedInfo?.number ?? ''}`
      : `Drag to draw a new ${drawType.name}`
  const trackHint = newType ? ` · Shift+click to track a new ${newType.name}` : ''
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
        cursor={drawType ? 'crosshair' : 'default'}
      >
        {frame && (
          <AnnotationLayer
            frameId={frame.id}
            ghost={ghost}
            imageSize={imageSize}
            scale={view.scale}
            tracks={tracks}
            draft={draft}
            draftColor={drawType?.color ?? '#a1a1aa'}
            pointer={drawType ? pointer : null}
          />
        )}
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
      <div className="pointer-events-none absolute top-3 left-3 flex items-center gap-2 rounded-md bg-zinc-900/80 px-2.5 py-1 text-xs/5 text-zinc-300 ring-1 ring-white/10 backdrop-blur">
        {drawType && (
          <span className="size-2 rounded-full" style={{ backgroundColor: drawType.color }} />
        )}
        {hint}
        {trackHint}
      </div>
    </>
  )
}
