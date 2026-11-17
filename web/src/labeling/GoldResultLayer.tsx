import { Fragment } from 'react'
import { Label, Rect, Tag, Text } from 'react-konva'
import type { GoldBox, GoldResult } from '@/gen/krill/v1/gold_pb'
import type { LabelType } from '@/gen/krill/v1/label_pb'
import { hasKeypoints } from '@/lib/keypoints'
import { konvaBox } from '@/lib/obb'
import type { Size } from '@/workspace/useWorkspaceStore'
import { outcomeColors, resultBoxes } from './goldResult'
import { KeypointMarks } from './KeypointMarks'

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
  const byId = new Map(types.map((t) => [t.id, t]))
  const shape = (b: GoldBox) =>
    b.box ? konvaBox(b.box, !!byId.get(b.labelTypeId)?.oriented, imageSize) : undefined
  const keypoints = (b: GoldBox, color: string, dashed: boolean) => {
    const type = byId.get(b.labelTypeId)
    if (!hasKeypoints(type) || !b.box?.keypoints.length) return null
    return (
      <KeypointMarks
        keypoints={b.box.keypoints}
        type={type}
        color={color}
        imageSize={imageSize}
        scale={scale}
        dashed={dashed}
      />
    )
  }
  return (
    <>
      {matched.map((b, i) => (
        <Fragment key={`a${i}`}>
          <Rect
            {...shape(b)}
            stroke="#ffffff"
            strokeWidth={1}
            dash={[6, 4]}
            strokeScaleEnabled={false}
            listening={false}
          />
          {keypoints(b, '#ffffff', true)}
        </Fragment>
      ))}
      {resultBoxes(result, types).map((r, i) => {
        const x = (r.box.box?.x ?? 0) * imageSize.width
        const y = (r.box.box?.y ?? 0) * imageSize.height
        const color = outcomeColors[r.outcome]
        return (
          <Fragment key={i}>
            <Rect
              {...shape(r.box)}
              stroke={color}
              strokeWidth={2}
              dash={r.answer ? [6, 4] : undefined}
              strokeScaleEnabled={false}
              listening={false}
            />
            {keypoints(r.box, color, r.answer)}
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
