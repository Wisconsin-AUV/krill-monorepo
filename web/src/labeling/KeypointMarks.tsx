import type Konva from 'konva'
import { Fragment } from 'react'
import { Circle, Group, Line, Text } from 'react-konva'
import { KeypointVisibility } from '@/gen/krill/v1/annotation_pb'
import type { LabelType } from '@/gen/krill/v1/label_pb'
import { derivedIndexes, isLabeled, padKeypoints, type KeypointValue } from '@/lib/keypoints'
import type { Size } from '@/workspace/useWorkspaceStore'

const RADIUS_PX = 4
const HANDLE_HIT_PX = 10
const TOP_EDGE_COLOR = '#ffffff'
const OUTLINE = '#09090b'

export interface KeypointEditing {
  active: number | null
  onPick: (index: number) => void
  onDragMove: (point: { x: number; y: number }) => void
  // In image pixels.
  onDragEnd: (index: number, point: { x: number; y: number }) => void
}

// A box's keypoints, quads, and edges. The quad edge from corner 0 to corner
// 1 is the surface's own top, so it is drawn apart from the rest.
export function KeypointMarks({
  keypoints,
  type,
  color,
  imageSize,
  scale,
  dashed = false,
  showIndex = false,
  editing,
}: {
  keypoints: readonly KeypointValue[]
  type: LabelType
  color: string
  imageSize: Size
  scale: number
  dashed?: boolean
  showIndex?: boolean
  editing?: KeypointEditing
}) {
  const kps = padKeypoints(keypoints, type.keypoints.length)
  const px = (i: number) => [kps[i].x * imageSize.width, kps[i].y * imageSize.height]
  const both = (a: number, b: number) => isLabeled(kps[a]) && isLabeled(kps[b])
  const dash = dashed ? [6, 4] : undefined
  const derived = derivedIndexes(type.quads)

  const lines: { from: number; to: number; top: boolean }[] = []
  for (const q of type.quads) {
    if (q.corners.length !== 4) continue
    q.corners.forEach((c, i) => lines.push({ from: c, to: q.corners[(i + 1) % 4], top: i === 0 }))
  }
  for (const e of type.edges) lines.push({ from: e.from, to: e.to, top: false })

  return (
    <Group listening={!!editing}>
      {lines.map(
        (l, i) =>
          both(l.from, l.to) && (
            <Line
              key={`l${i}`}
              points={[...px(l.from), ...px(l.to)]}
              stroke={l.top ? TOP_EDGE_COLOR : color}
              strokeWidth={l.top ? 2.5 : 1.5}
              dash={dash}
              strokeScaleEnabled={false}
              listening={false}
            />
          ),
      )}
      {kps.map((k, i) => {
        if (!isLabeled(k)) return null
        const [x, y] = px(i)
        const hidden = k.visibility === KeypointVisibility.HIDDEN
        const handle = !!editing && !derived.has(i)
        return (
          <Fragment key={i}>
            {editing?.active === i && (
              <Circle
                x={x}
                y={y}
                radius={(RADIUS_PX + 4) / scale}
                stroke={TOP_EDGE_COLOR}
                strokeWidth={1.5}
                strokeScaleEnabled={false}
                listening={false}
              />
            )}
            <Circle
              x={x}
              y={y}
              radius={RADIUS_PX / scale}
              fill={hidden ? 'transparent' : color}
              stroke={hidden ? color : OUTLINE}
              strokeWidth={hidden ? 2 : 1}
              dash={hidden ? dash : undefined}
              strokeScaleEnabled={false}
              hitStrokeWidth={HANDLE_HIT_PX}
              listening={handle}
              draggable={handle}
              onMouseDown={(e: Konva.KonvaEventObject<MouseEvent>) => {
                if (e.evt.button !== 0 || e.evt.altKey) return
                e.cancelBubble = true
                editing?.onPick(i)
              }}
              onMouseEnter={(e: Konva.KonvaEventObject<MouseEvent>) => {
                const c = e.target.getStage()?.container()
                if (c) c.style.cursor = 'grab'
              }}
              onMouseLeave={(e: Konva.KonvaEventObject<MouseEvent>) => {
                const c = e.target.getStage()?.container()
                if (c) c.style.cursor = ''
              }}
              onDragMove={(e: Konva.KonvaEventObject<DragEvent>) =>
                editing?.onDragMove({ x: e.target.x(), y: e.target.y() })
              }
              onDragEnd={(e: Konva.KonvaEventObject<DragEvent>) => {
                const to = { x: e.target.x(), y: e.target.y() }
                // Put back until the store redraws it, so a failed save
                // doesn't leave it where it was dropped.
                e.target.position({ x, y })
                editing?.onDragEnd(i, to)
              }}
            />
            {showIndex && (
              <Text
                x={x}
                y={y}
                offsetX={-RADIUS_PX - 2}
                offsetY={RADIUS_PX + 12}
                scaleX={1 / scale}
                scaleY={1 / scale}
                text={String(i + 1)}
                fontSize={10}
                fontStyle="600"
                fill="#ffffff"
                shadowColor="#000000"
                shadowBlur={2}
                shadowOpacity={0.9}
                listening={false}
              />
            )}
          </Fragment>
        )
      })}
    </Group>
  )
}
