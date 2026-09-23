import {
  ChevronDoubleLeftIcon,
  ChevronDoubleRightIcon,
  ExclamationTriangleIcon,
  TrashIcon,
} from '@heroicons/react/16/solid'
import { clsx } from 'clsx'
import { useMemo } from 'react'
import { Button } from '@/components/ui/Button'
import { Select } from '@/components/ui/Select'
import type { LabelType } from '@/gen/krill/v1/label_pb'
import { plural } from '@/lib/format'
import { missingAttributes } from '@/lib/labels'
import { useWorkspaceStore } from '@/workspace/useWorkspaceStore'
import type { TrackInfo } from './AnnotationLayer'
import { idKey, useLabelStore } from './useLabelStore'

function TrackEditor({
  info,
  types,
  positions,
}: {
  info: TrackInfo
  types: LabelType[]
  positions: number[]
}) {
  const updateTrack = useLabelStore((s) => s.updateTrack)
  const deleteTrack = useLabelStore((s) => s.deleteTrack)
  const seek = useWorkspaceStore((s) => s.seek)
  const { track, type } = info

  return (
    <div className="space-y-3 border-t border-white/5 px-3 pt-3 pb-3">
      <label className="block">
        <span className="text-xs/6 text-zinc-400">Type</span>
        <Select
          value={String(track.labelTypeId)}
          onChange={(e) => void updateTrack(track.id, { labelTypeId: BigInt(e.target.value) })}
        >
          {types.map((t) => (
            <option key={String(t.id)} value={String(t.id)}>
              {t.name}
            </option>
          ))}
        </Select>
      </label>
      {type?.attributes.map((a) => (
        <label key={a.name} className="block">
          <span className="text-xs/6 text-zinc-400">{a.name}</span>
          <Select
            value={track.attributes[a.name] ?? ''}
            onChange={(e) => {
              const next = { ...track.attributes }
              if (e.target.value) next[a.name] = e.target.value
              else delete next[a.name]
              void updateTrack(track.id, { attributes: next })
            }}
          >
            <option value="">Not set</option>
            {a.options.map((o) => (
              <option key={o} value={o}>
                {o}
              </option>
            ))}
          </Select>
        </label>
      ))}
      <div className="flex items-center gap-1 pt-1">
        <Button plain title="First box" onClick={() => positions.length && seek(positions[0])}>
          <ChevronDoubleLeftIcon data-slot="icon" />
        </Button>
        <Button
          plain
          title="Last box"
          onClick={() => positions.length && seek(positions[positions.length - 1])}
        >
          <ChevronDoubleRightIcon data-slot="icon" />
        </Button>
        <div className="flex-1" />
        <Button
          plain
          title="Delete track (Shift+Delete)"
          onClick={() => void deleteTrack(track.id)}
        >
          <TrashIcon data-slot="icon" />
          Delete track
        </Button>
      </div>
    </div>
  )
}

export function TrackPanel({
  tracks,
  types,
}: {
  tracks: Map<string, TrackInfo>
  types: LabelType[]
}) {
  const frames = useWorkspaceStore((s) => s.frames)
  const index = useWorkspaceStore((s) => s.index)
  const boxes = useLabelStore((s) => s.boxes)
  const selectedTrackId = useLabelStore((s) => s.selectedTrackId)
  const select = useLabelStore((s) => s.select)

  // Frame positions (within the clip) that each track has a box on.
  const positions = useMemo(() => {
    const out = new Map<string, number[]>()
    frames.forEach((f, i) => {
      for (const t of Object.keys(boxes[idKey(f.id)] ?? {})) {
        const list = out.get(t)
        if (list) list.push(i)
        else out.set(t, [i])
      }
    })
    return out
  }, [boxes, frames])

  const current = frames[index]
  const here = boxes[current ? idKey(current.id) : ''] ?? {}
  const list = [...tracks.values()]

  return (
    <aside className="flex w-72 shrink-0 flex-col border-l border-white/10 bg-zinc-900">
      <div className="flex items-baseline justify-between px-3 pt-3 pb-2">
        <h2 className="text-xs/6 font-medium text-zinc-400">Tracks in clip</h2>
        <span className="text-xs text-zinc-500 tabular-nums">{list.length}</span>
      </div>
      <ul className="flex-1 overflow-y-auto pb-3">
        {list.map((info) => {
          const k = idKey(info.track.id)
          const selected = info.track.id === selectedTrackId
          const missing = missingAttributes(info.type, info.track.attributes)
          const count = positions.get(k)?.length ?? 0
          return (
            <li key={k} className={clsx(selected && 'bg-white/5')}>
              <button
                type="button"
                onClick={() => select(selected ? null : info.track.id)}
                className="flex w-full items-center gap-2.5 px-3 py-2 text-left text-sm/5 hover:bg-white/5"
              >
                <span
                  className="size-2.5 shrink-0 rounded-sm"
                  style={{ backgroundColor: info.type?.color ?? '#a1a1aa' }}
                />
                <span className="min-w-0 flex-1 truncate">
                  <span className="font-mono text-[13px] text-zinc-100">
                    {info.type?.name ?? 'unknown'}
                  </span>
                  <span className="text-zinc-500"> #{info.number}</span>
                </span>
                {missing.length > 0 && (
                  <ExclamationTriangleIcon
                    className="size-4 shrink-0 text-amber-400"
                    aria-label={`Missing ${missing.join(', ')}`}
                  />
                )}
                <span
                  className="text-xs text-zinc-500 tabular-nums"
                  title={plural(count, 'box', 'boxes')}
                >
                  {count}
                </span>
                <span
                  className={clsx(
                    'size-1.5 shrink-0 rounded-full',
                    here[k] ? 'bg-sky-400' : 'bg-transparent',
                  )}
                  title={here[k] ? 'Has a box on this frame' : undefined}
                />
              </button>
              {selected && (
                <TrackEditor info={info} types={types} positions={positions.get(k) ?? []} />
              )}
            </li>
          )
        })}
        {list.length === 0 && (
          <li className="px-3 py-4 text-sm/6 text-zinc-400">
            Pick a type and drag on the frame to draw a box. Each object becomes a track you carry
            across frames.
          </li>
        )}
      </ul>
    </aside>
  )
}
