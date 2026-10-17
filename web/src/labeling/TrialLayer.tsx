import { Fragment, useMemo } from 'react'
import { Group, Label, Rect, Tag, Text } from 'react-konva'
import type { LabelType } from '@/gen/krill/v1/label_pb'
import type { ModelTrialBox } from '@/gen/krill/v1/pipeline_pb'
import { useWorkspaceStore } from '@/workspace/useWorkspaceStore'
import { outcomeColors } from './goldResult'
import { above, isDone, matchFrame, verifiedBoxes } from './trialMatch'
import { idKey, useLabelStore, type BoxRect } from './useLabelStore'
import { useTrialStore } from './useTrialStore'

const FALLBACK_COLOR = '#a1a1aa'

interface Mark {
  box: BoxRect
  color: string
  caption: string
  dashed: boolean
}

const pct = (b: ModelTrialBox) => `${Math.round(b.confidence * 100)}%`

// The model's boxes for the current frame. Never listening and never
// transformable, so they can't be mistaken for or edited like real boxes.
export function TrialLayer({ types }: { types: LabelType[] }) {
  const frame = useWorkspaceStore((s) => s.frames[s.index])
  const index = useWorkspaceStore((s) => s.index)
  const imageSize = useWorkspaceStore((s) => s.imageSize)
  const scale = useWorkspaceStore((s) => s.view.scale)
  const trial = useTrialStore((s) => s.trial)
  const found = useTrialStore((s) => (frame ? s.boxes[idKey(frame.id)] : undefined))
  const threshold = useTrialStore((s) => s.threshold)
  const mode = useTrialStore((s) => s.mode)
  const visible = useTrialStore((s) => s.visible)
  const labeled = useLabelStore((s) => (frame ? s.boxes[idKey(frame.id)] : undefined))
  const tracks = useLabelStore((s) => s.tracks)
  const status = useLabelStore((s) => (frame ? s.frameStatus[idKey(frame.id)] : undefined))

  const marks = useMemo((): Mark[] => {
    const byId = new Map(types.map((t) => [t.id, t]))
    const name = (id: bigint) => byId.get(id)?.name ?? 'unknown'
    const model = above(found, threshold)
    const compare = mode === 'compare' && isDone(status) && !!trial && index < trial.framesDone
    if (!compare) {
      return model.flatMap((b) => {
        if (!b.box) return []
        const type = byId.get(b.labelTypeId)
        const attrs = type?.attributes.map((a) => b.attributes[a.name]).filter(Boolean) ?? []
        return [
          {
            box: b.box,
            color: type?.color ?? FALLBACK_COLOR,
            caption: `${[type?.name ?? 'unknown', ...attrs].join(' ')} · ${pct(b)}`,
            dashed: true,
          },
        ]
      })
    }
    const verified = verifiedBoxes(labeled, tracks)
    const m = matchFrame(model, verified)
    const out: Mark[] = []
    for (const p of m.pairs) {
      const b = model[p.model]
      const why = p.correct ? '' : p.wrong ? ` · wrong ${p.wrong}` : ` · IoU ${p.iou.toFixed(2)}`
      out.push({
        box: b.box!,
        color: outcomeColors[p.correct ? 'correct' : 'off'],
        caption: `${name(b.labelTypeId)}${why} · ${pct(b)}`,
        dashed: true,
      })
    }
    for (const j of m.missed) {
      const v = verified[j]
      out.push({
        box: v.box,
        color: outcomeColors.missed,
        caption: `${name(v.labelTypeId)} · missed`,
        dashed: false,
      })
    }
    for (const i of m.extra) {
      const b = model[i]
      if (!b.box) continue
      out.push({
        box: b.box,
        color: outcomeColors.extra,
        caption: `${name(b.labelTypeId)} · extra · ${pct(b)}`,
        dashed: true,
      })
    }
    return out
  }, [types, found, threshold, mode, status, trial, index, labeled, tracks])

  if (!trial || !visible) return null
  return (
    <Group listening={false}>
      {marks.map((k, i) => {
        const x = k.box.x * imageSize.width
        const y = k.box.y * imageSize.height
        const height = k.box.height * imageSize.height
        return (
          <Fragment key={i}>
            <Rect
              x={x}
              y={y}
              width={k.box.width * imageSize.width}
              height={height}
              stroke={k.color}
              strokeWidth={2}
              dash={k.dashed ? [6, 4] : undefined}
              strokeScaleEnabled={false}
            />
            {/* Below the box, so it never covers the caption of a real box. */}
            <Label x={x} y={y + height} scaleX={1 / scale} scaleY={1 / scale}>
              <Tag fill={k.color} cornerRadius={3} />
              <Text text={k.caption} fontSize={12} fontStyle="600" padding={4} fill="#09090b" />
            </Label>
          </Fragment>
        )
      })}
    </Group>
  )
}
