import {
  ChevronDownIcon,
  ChevronUpIcon,
  PlusIcon,
  Squares2X2Icon,
  XMarkIcon,
} from '@heroicons/react/20/solid'
import { useState } from 'react'
import { Button } from '@/components/ui/Button'
import { Checkbox, CheckboxField } from '@/components/ui/Checkbox'
import { Description, Fieldset, Label, Legend } from '@/components/ui/Fieldset'
import { Input } from '@/components/ui/Input'
import { Select } from '@/components/ui/Select'
import {
  draftId,
  type EdgeDraft,
  type KeypointDraft,
  type KeypointsDraft,
  type QuadDraft,
} from '@/components/keypointsDraft'

const cornerSuffixes = ['tl', 'tr', 'br', 'bl']
const cornerLabels = ['Top left', 'Top right', 'Bottom right', 'Bottom left']

function KeypointSelect({
  keypoints,
  value,
  onChange,
  empty,
  ...props
}: {
  keypoints: KeypointDraft[]
  value: number | null
  onChange: (id: number | null) => void
  empty: string
  'aria-label': string
  className?: string
}) {
  return (
    <Select
      {...props}
      value={value === null ? '' : String(value)}
      onChange={(e) => onChange(e.target.value === '' ? null : Number(e.target.value))}
    >
      <option value="">{empty}</option>
      {keypoints.map((k, i) => (
        <option key={k.id} value={String(k.id)}>
          {i + 1} {k.name}
        </option>
      ))}
    </Select>
  )
}

// The quad as designed, numbered by keypoint, with its top edge highlighted.
function QuadSchematic({ quad, keypoints }: { quad: QuadDraft; keypoints: KeypointDraft[] }) {
  const pts = [
    [16, 16],
    [48, 16],
    [48, 48],
    [16, 48],
  ] as const
  const offsets = [
    [-9, -6],
    [9, -6],
    [9, 12],
    [-9, 12],
  ] as const
  const number = (id: number | null) => {
    const i = keypoints.findIndex((k) => k.id === id)
    return i < 0 ? '?' : String(i + 1)
  }
  return (
    <svg viewBox="0 0 64 64" className="size-16 shrink-0" aria-hidden>
      <polygon
        points={pts.map((p) => p.join(',')).join(' ')}
        className="fill-zinc-950/5 stroke-zinc-400 dark:fill-white/5"
        strokeWidth={1.5}
      />
      <line x1={16} y1={16} x2={48} y2={16} className="stroke-sky-500" strokeWidth={3} />
      {quad.center !== null && (
        <>
          <line x1={16} y1={16} x2={48} y2={48} className="stroke-zinc-300 dark:stroke-zinc-600" />
          <line x1={48} y1={16} x2={16} y2={48} className="stroke-zinc-300 dark:stroke-zinc-600" />
          <circle cx={32} cy={32} r={2} className="fill-zinc-500" />
        </>
      )}
      {pts.map(([x, y], i) => (
        <text
          key={i}
          x={x + offsets[i]![0]}
          y={y + offsets[i]![1]}
          textAnchor="middle"
          className="fill-zinc-600 text-[9px] tabular-nums dark:fill-zinc-300"
        >
          {number(quad.corners[i] ?? null)}
        </text>
      ))}
    </svg>
  )
}

export function KeypointsEditor({
  draft,
  onChange,
}: {
  draft: KeypointsDraft
  onChange: (d: KeypointsDraft) => void
}) {
  const [prefix, setPrefix] = useState('')
  const [center, setCenter] = useState(false)
  const { keypoints, quads, edges } = draft

  const setKeypoints = (k: KeypointDraft[]) => onChange({ ...draft, keypoints: k })
  const patchQuad = (id: number, p: Partial<QuadDraft>) =>
    onChange({ ...draft, quads: quads.map((q) => (q.id === id ? { ...q, ...p } : q)) })
  const patchEdge = (id: number, p: Partial<EdgeDraft>) =>
    onChange({ ...draft, edges: edges.map((e) => (e.id === id ? { ...e, ...p } : e)) })

  function setMirror(id: number, partner: number | null) {
    setKeypoints(
      keypoints.map((k) => {
        if (k.id === id) return { ...k, mirror: partner }
        if (partner !== null && k.id === partner) return { ...k, mirror: id }
        if (k.mirror === id || (partner !== null && k.mirror === partner))
          return { ...k, mirror: null }
        return k
      }),
    )
  }

  function move(i: number, by: number) {
    const next = [...keypoints]
    const [k] = next.splice(i, 1)
    next.splice(i + by, 0, k!)
    setKeypoints(next)
  }

  function remove(id: number) {
    const drop = (c: number | null) => (c === id ? null : c)
    onChange({
      keypoints: keypoints
        .filter((k) => k.id !== id)
        .map((k) => ({ ...k, mirror: drop(k.mirror) })),
      quads: quads.map((q) => ({ ...q, corners: q.corners.map(drop), center: drop(q.center) })),
      edges: edges.filter((e) => e.from !== id && e.to !== id),
    })
  }

  function addQuad() {
    const p = prefix.trim().toLowerCase()
    const corners = cornerSuffixes.map((s) => ({
      id: draftId(),
      name: `${p}_${s}`,
      mirror: null,
      origin: null,
    }))
    const mid = center ? { id: draftId(), name: `${p}_center`, mirror: null, origin: null } : null
    onChange({
      ...draft,
      keypoints: [...keypoints, ...corners, ...(mid ? [mid] : [])],
      quads: [
        ...quads,
        {
          id: draftId(),
          corners: corners.map((c) => c.id),
          reference: false,
          center: mid?.id ?? null,
        },
      ],
    })
    setPrefix('')
  }

  return (
    <Fieldset>
      <Legend>Keypoints</Legend>
      <Description>Exported as YOLO-pose.</Description>

      <div className="mt-4 space-y-2">
        {keypoints.map((k, i) => (
          <div key={k.id} className="flex items-center gap-2">
            <span className="w-5 shrink-0 text-right text-sm/6 text-zinc-500 tabular-nums dark:text-zinc-400">
              {i + 1}
            </span>
            <Input
              aria-label="Keypoint name"
              placeholder="nose"
              value={k.name}
              onChange={(e) =>
                setKeypoints(
                  keypoints.map((x) => (x.id === k.id ? { ...x, name: e.target.value } : x)),
                )
              }
              maxLength={40}
              autoComplete="off"
            />
            <KeypointSelect
              aria-label="Mirror"
              empty="No mirror"
              className="max-w-40"
              keypoints={keypoints.filter((x) => x.id !== k.id)}
              value={k.mirror}
              onChange={(m) => setMirror(k.id, m)}
            />
            <Button plain aria-label="Move up" disabled={i === 0} onClick={() => move(i, -1)}>
              <ChevronUpIcon data-slot="icon" />
            </Button>
            <Button
              plain
              aria-label="Move down"
              disabled={i === keypoints.length - 1}
              onClick={() => move(i, 1)}
            >
              <ChevronDownIcon data-slot="icon" />
            </Button>
            <Button plain aria-label="Remove keypoint" onClick={() => remove(k.id)}>
              <XMarkIcon data-slot="icon" />
            </Button>
          </div>
        ))}
        <div className="flex flex-wrap items-center gap-3 pt-1">
          <Button
            outline
            onClick={() =>
              setKeypoints([...keypoints, { id: draftId(), name: '', mirror: null, origin: null }])
            }
          >
            <PlusIcon data-slot="icon" />
            Add keypoint
          </Button>
          <div className="flex items-center gap-3">
            <Input
              aria-label="Quad prefix"
              placeholder="paper"
              value={prefix}
              onChange={(e) => setPrefix(e.target.value)}
              maxLength={30}
              autoComplete="off"
              className="max-w-32"
            />
            <CheckboxField>
              <Checkbox checked={center} onChange={setCenter} color="sky" />
              <Label>Center</Label>
            </CheckboxField>
            <Button outline disabled={!prefix.trim()} onClick={addQuad}>
              <Squares2X2Icon data-slot="icon" />
              Add quad
            </Button>
          </div>
        </div>
      </div>

      {quads.length > 0 && (
        <div className="mt-6">
          <p className="text-sm/6 font-medium text-zinc-950 dark:text-white">Quads</p>
          <ul className="mt-2 space-y-4">
            {quads.map((q) => (
              <li key={q.id} className="flex items-start gap-3">
                <QuadSchematic quad={q} keypoints={keypoints} />
                <div className="min-w-0 flex-1 space-y-2">
                  <div className="grid grid-cols-2 gap-2">
                    {[0, 1, 3, 2].map((c) => (
                      <KeypointSelect
                        key={c}
                        aria-label={`${cornerLabels[c]} corner`}
                        empty={cornerLabels[c]!}
                        keypoints={keypoints}
                        value={q.corners[c] ?? null}
                        onChange={(id) =>
                          patchQuad(q.id, { corners: q.corners.map((x, j) => (j === c ? id : x)) })
                        }
                      />
                    ))}
                  </div>
                  <div className="flex items-center gap-3">
                    <KeypointSelect
                      aria-label="Center"
                      empty="No center"
                      className="max-w-48"
                      keypoints={keypoints}
                      value={q.center}
                      onChange={(id) => patchQuad(q.id, { center: id })}
                    />
                    <CheckboxField>
                      <Checkbox
                        checked={q.reference}
                        color="sky"
                        onChange={(on) =>
                          onChange({
                            ...draft,
                            quads: quads.map((x) => ({
                              ...x,
                              reference: x.id === q.id ? on : on ? false : x.reference,
                            })),
                          })
                        }
                      />
                      <Label>Reference</Label>
                    </CheckboxField>
                    <Button
                      plain
                      aria-label="Remove quad"
                      className="ml-auto"
                      onClick={() =>
                        onChange({ ...draft, quads: quads.filter((x) => x.id !== q.id) })
                      }
                    >
                      <XMarkIcon data-slot="icon" />
                    </Button>
                  </div>
                </div>
              </li>
            ))}
          </ul>
        </div>
      )}

      {keypoints.length > 1 && (
        <div className="mt-6">
          <p className="text-sm/6 font-medium text-zinc-950 dark:text-white">Edges</p>
          <div className="mt-2 space-y-2">
            {edges.map((e) => (
              <div key={e.id} className="flex items-center gap-2">
                <KeypointSelect
                  aria-label="From"
                  empty="From"
                  keypoints={keypoints}
                  value={e.from}
                  onChange={(id) => patchEdge(e.id, { from: id })}
                />
                <KeypointSelect
                  aria-label="To"
                  empty="To"
                  keypoints={keypoints}
                  value={e.to}
                  onChange={(id) => patchEdge(e.id, { to: id })}
                />
                <Button
                  plain
                  aria-label="Remove edge"
                  onClick={() => onChange({ ...draft, edges: edges.filter((x) => x.id !== e.id) })}
                >
                  <XMarkIcon data-slot="icon" />
                </Button>
              </div>
            ))}
            <Button
              outline
              onClick={() =>
                onChange({ ...draft, edges: [...edges, { id: draftId(), from: null, to: null }] })
              }
            >
              <PlusIcon data-slot="icon" />
              Add edge
            </Button>
          </div>
        </div>
      )}
    </Fieldset>
  )
}
