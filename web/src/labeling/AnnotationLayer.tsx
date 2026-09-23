import type Konva from 'konva'
import { Fragment, useEffect, useMemo, useRef } from 'react'
import { Group, Label, Rect, Tag, Text, Transformer } from 'react-konva'
import type { Annotation, Track } from '@/gen/krill/v1/annotation_pb'
import type { LabelType } from '@/gen/krill/v1/label_pb'
import { missingAttributes, withAlpha } from '@/lib/labels'
import type { Size } from '@/workspace/useWorkspaceStore'
import { idKey, useLabelStore, type BoxRect } from './useLabelStore'

const MIN_SIDE_PX = 3
const FALLBACK_COLOR = '#a1a1aa'

function toPixels(box: BoxRect, size: Size) {
  return {
    x: box.x * size.width,
    y: box.y * size.height,
    width: box.width * size.width,
    height: box.height * size.height,
  }
}

function toNormalized(node: Konva.Node, size: Size): BoxRect {
  const w = Math.max(node.width() * node.scaleX(), 1)
  const h = Math.max(node.height() * node.scaleY(), 1)
  return {
    x: node.x() / size.width,
    y: node.y() / size.height,
    width: w / size.width,
    height: h / size.height,
  }
}

export interface TrackInfo {
  number: number
  track: Track
  type?: LabelType
}

export function AnnotationLayer({
  frameId,
  ghost,
  imageSize,
  scale,
  tracks,
  draft,
  draftColor,
  pointer,
}: {
  frameId: bigint
  ghost?: Annotation
  imageSize: Size
  scale: number
  tracks: Map<string, TrackInfo>
  draft: BoxRect | null
  draftColor: string
  pointer: { x: number; y: number } | null
}) {
  const byTrack = useLabelStore((s) => s.boxes[idKey(frameId)])
  const selectedTrackId = useLabelStore((s) => s.selectedTrackId)
  const hidden = useLabelStore((s) => s.hideBoxes)
  const select = useLabelStore((s) => s.select)
  const moveBox = useLabelStore((s) => s.moveBox)
  const transformer = useRef<Konva.Transformer>(null)
  const selectedRect = useRef<Konva.Rect>(null)

  const boxes = useMemo(() => Object.values(byTrack ?? {}), [byTrack])
  const selectedBox = boxes.find((b) => b.trackId === selectedTrackId)
  const stroke = 2 / scale

  useEffect(() => {
    const tr = transformer.current
    if (!tr) return
    tr.nodes(selectedRect.current && !hidden ? [selectedRect.current] : [])
    tr.getLayer()?.batchDraw()
  })

  const commit = (a: Annotation, node: Konva.Node) => {
    const box = toNormalized(node, imageSize)
    node.scaleX(1)
    node.scaleY(1)
    void moveBox(a.trackId, a.frameId, box)
  }

  return (
    <>
      {pointer && (
        <Group listening={false}>
          <Rect x={0} y={pointer.y} width={imageSize.width} height={1 / scale} fill="#ffffff66" />
          <Rect x={pointer.x} y={0} width={1 / scale} height={imageSize.height} fill="#ffffff66" />
        </Group>
      )}

      {ghost &&
        !hidden &&
        (() => {
          const info = tracks.get(idKey(ghost.trackId))
          const color = info?.type?.color ?? FALLBACK_COLOR
          const p = toPixels(ghost.box!, imageSize)
          return (
            <Rect
              {...p}
              stroke={color}
              strokeWidth={stroke}
              dash={[6 / scale, 4 / scale]}
              opacity={0.7}
              listening={false}
            />
          )
        })()}

      {!hidden &&
        boxes.map((a) => {
          const info = tracks.get(idKey(a.trackId))
          const color = info?.type?.color ?? FALLBACK_COLOR
          const selected = a.trackId === selectedTrackId
          const p = toPixels(a.box!, imageSize)
          const missing = info
            ? missingAttributes(info.type, info.track.attributes).length > 0
            : false
          const caption = `${info?.type?.name ?? 'unknown'} #${info?.number ?? '?'}${missing ? ' ⚠' : ''}`
          return (
            <Fragment key={idKey(a.trackId)}>
              <Rect
                ref={selected ? selectedRect : undefined}
                {...p}
                stroke={color}
                strokeWidth={selected ? 3 : 2}
                fill={withAlpha(color, selected ? 0.18 : 0.06)}
                strokeScaleEnabled={false}
                draggable
                onMouseDown={(e) => {
                  if (e.evt.button !== 0 || e.evt.altKey) return
                  e.cancelBubble = true
                  select(a.trackId)
                }}
                onMouseEnter={(e) => {
                  const c = e.target.getStage()?.container()
                  if (c) c.style.cursor = 'move'
                }}
                onMouseLeave={(e) => {
                  const c = e.target.getStage()?.container()
                  if (c) c.style.cursor = ''
                }}
                onDragEnd={(e) => commit(a, e.target)}
                onTransformEnd={(e) => commit(a, e.target)}
              />
              <Label
                x={p.x}
                y={p.y}
                scaleX={1 / scale}
                scaleY={1 / scale}
                offsetY={20}
                listening={false}
              >
                <Tag fill={color} cornerRadius={3} />
                <Text text={caption} fontSize={12} fontStyle="600" padding={4} fill="#09090b" />
              </Label>
            </Fragment>
          )
        })}

      {draft && (
        <Rect
          {...toPixels(draft, imageSize)}
          stroke={draftColor}
          strokeWidth={stroke}
          fill={withAlpha(draftColor, 0.12)}
          dash={[4 / scale, 3 / scale]}
          listening={false}
        />
      )}

      <Transformer
        ref={transformer}
        rotateEnabled={false}
        flipEnabled={false}
        keepRatio={false}
        ignoreStroke
        anchorSize={8}
        anchorCornerRadius={2}
        borderStroke={
          selectedBox
            ? (tracks.get(idKey(selectedBox.trackId))?.type?.color ?? FALLBACK_COLOR)
            : FALLBACK_COLOR
        }
        anchorStroke="#ffffff"
        anchorFill="#18181b"
        boundBoxFunc={(oldBox, newBox) =>
          newBox.width < MIN_SIDE_PX || newBox.height < MIN_SIDE_PX ? oldBox : newBox
        }
      />
    </>
  )
}
