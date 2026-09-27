import { Fragment } from 'react'
import { Label, Rect, Tag, Text } from 'react-konva'
import type { GoldResult } from '@/gen/krill/v1/gold_pb'
import type { LabelType } from '@/gen/krill/v1/label_pb'
import type { Size } from '@/workspace/useWorkspaceStore'
import { outcomeColors, resultBoxes } from './goldResult'

export function GoldResultLayer({
  result,
  types,
  imageSize,
  scale,
}: {
  result: GoldResult
  types: LabelType[]
  imageSize: Size
  scale: number
}) {
  const matched = result.matches.map((m) => result.answer[m.answer])
  return (
    <>
      {matched.map((b, i) => (
        <Rect
          key={`a${i}`}
          x={(b.box?.x ?? 0) * imageSize.width}
          y={(b.box?.y ?? 0) * imageSize.height}
          width={(b.box?.width ?? 0) * imageSize.width}
          height={(b.box?.height ?? 0) * imageSize.height}
          stroke="#ffffff"
          strokeWidth={1}
          dash={[6, 4]}
          strokeScaleEnabled={false}
          listening={false}
        />
      ))}
      {resultBoxes(result, types).map((r, i) => {
        const x = (r.box.box?.x ?? 0) * imageSize.width
        const y = (r.box.box?.y ?? 0) * imageSize.height
        const color = outcomeColors[r.outcome]
        return (
          <Fragment key={i}>
            <Rect
              x={x}
              y={y}
              width={(r.box.box?.width ?? 0) * imageSize.width}
              height={(r.box.box?.height ?? 0) * imageSize.height}
              stroke={color}
              strokeWidth={2}
              dash={r.answer ? [6, 4] : undefined}
              strokeScaleEnabled={false}
              listening={false}
            />
            <Label x={x} y={y} scaleX={1 / scale} scaleY={1 / scale} offsetY={20} listening={false}>
              <Tag fill={color} cornerRadius={3} />
              <Text text={r.caption} fontSize={12} fontStyle="600" padding={4} fill="#09090b" />
            </Label>
          </Fragment>
        )
      })}
    </>
  )
}
