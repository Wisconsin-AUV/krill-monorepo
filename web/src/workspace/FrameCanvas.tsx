import type Konva from 'konva'
import { useEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import { Group, Image as KonvaImage, Layer, Stage } from 'react-konva'
import LoadingSpinner from '@/components/ui/LoadingSpinner'
import { loadFrame } from './imageCache'
import { useWorkspaceStore } from './useWorkspaceStore'

const PRELOAD_AHEAD = 12
const PRELOAD_BEHIND = 4

export function FrameCanvas({
  children,
  onStageMouseDown,
  onPointerMove,
  cursor,
}: {
  children?: ReactNode
  onStageMouseDown?: (e: Konva.KonvaEventObject<MouseEvent>) => void
  onPointerMove?: (point: { x: number; y: number } | null) => void
  cursor?: string
}) {
  const container = useRef<HTMLDivElement>(null)
  const frames = useWorkspaceStore((s) => s.frames)
  const index = useWorkspaceStore((s) => s.index)
  const view = useWorkspaceStore((s) => s.view)
  const stageSize = useWorkspaceStore((s) => s.stageSize)
  const imageSize = useWorkspaceStore((s) => s.imageSize)
  const setStageSize = useWorkspaceStore((s) => s.setStageSize)
  const setView = useWorkspaceStore((s) => s.setView)
  const zoomAt = useWorkspaceStore((s) => s.zoomAt)
  const [shown, setShown] = useState<HTMLImageElement | null>(null)
  const pan = useRef<{ x: number; y: number } | null>(null)

  const frame = frames[index]
  const entry = useMemo(() => (frame ? loadFrame(frame.id, frame.url) : undefined), [frame])

  useEffect(() => {
    if (!entry) return
    let live = true
    entry.ready.then(
      (img) => live && setShown(img),
      () => undefined,
    )
    return () => {
      live = false
    }
  }, [entry])

  useEffect(() => {
    for (let i = index + 1; i <= index + PRELOAD_AHEAD && i < frames.length; i++) {
      loadFrame(frames[i].id, frames[i].url)
    }
    for (let i = index - 1; i >= index - PRELOAD_BEHIND && i >= 0; i--) {
      loadFrame(frames[i].id, frames[i].url)
    }
  }, [frames, index])

  useEffect(() => {
    const el = container.current
    if (!el) return
    const observer = new ResizeObserver(([e]) =>
      setStageSize({ width: e.contentRect.width, height: e.contentRect.height }),
    )
    observer.observe(el)
    return () => observer.disconnect()
  }, [setStageSize])

  // Until the new frame decodes, keep drawing the previous one so stepping
  // through frames never flashes an empty canvas.
  const image = entry?.loaded ? entry.img : shown
  const loading = !!frame && !entry?.loaded

  function onWheel(e: Konva.KonvaEventObject<WheelEvent>) {
    e.evt.preventDefault()
    const pointer = e.target.getStage()?.getPointerPosition() ?? undefined
    // Pinch gestures arrive as ctrl+wheel with small deltas, so amplify them.
    const speed = e.evt.ctrlKey ? 0.01 : 0.0015
    zoomAt(Math.exp(-e.evt.deltaY * speed), pointer)
  }

  function onMouseDown(e: Konva.KonvaEventObject<MouseEvent>) {
    if (e.evt.button === 1 || e.evt.button === 2 || (e.evt.button === 0 && e.evt.altKey)) {
      e.evt.preventDefault()
      pan.current = { x: e.evt.clientX, y: e.evt.clientY }
      return
    }
    onStageMouseDown?.(e)
  }

  function onMouseMove(e: Konva.KonvaEventObject<MouseEvent>) {
    if (!pan.current) {
      onPointerMove?.(e.target.getStage()?.getPointerPosition() ?? null)
      return
    }
    const { x, y } = useWorkspaceStore.getState().view
    setView({
      scale: view.scale,
      x: x + e.evt.clientX - pan.current.x,
      y: y + e.evt.clientY - pan.current.y,
    })
    pan.current = { x: e.evt.clientX, y: e.evt.clientY }
  }

  return (
    <div
      ref={container}
      className="absolute inset-0 overflow-hidden"
      style={{ cursor }}
      onContextMenu={(e) => e.preventDefault()}
    >
      {stageSize.width > 0 && (
        <Stage
          width={stageSize.width}
          height={stageSize.height}
          onWheel={onWheel}
          onMouseDown={onMouseDown}
          onMouseMove={onMouseMove}
          onMouseUp={() => (pan.current = null)}
          onMouseLeave={() => {
            pan.current = null
            onPointerMove?.(null)
          }}
        >
          <Layer listening={false}>
            <Group x={view.x} y={view.y} scaleX={view.scale} scaleY={view.scale}>
              {image && (
                <KonvaImage image={image} width={imageSize.width} height={imageSize.height} />
              )}
            </Group>
          </Layer>
          <Layer>
            <Group x={view.x} y={view.y} scaleX={view.scale} scaleY={view.scale}>
              {children}
            </Group>
          </Layer>
        </Stage>
      )}
      {loading && (
        <div className="pointer-events-none absolute top-3 right-3 rounded-full bg-zinc-900/80 p-1.5 ring-1 ring-white/10">
          <LoadingSpinner />
        </div>
      )}
    </div>
  )
}
