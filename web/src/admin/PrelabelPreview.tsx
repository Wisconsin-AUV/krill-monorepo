import { useQuery } from '@connectrpc/connect-query'
import { keepPreviousData } from '@tanstack/react-query'
import { useEffect, useState } from 'react'
import { Callout } from '@/components/ui/Callout'
import { Field, Label } from '@/components/ui/Fieldset'
import LoadingSpinner from '@/components/ui/LoadingSpinner'
import { Select } from '@/components/ui/Select'
import { Switch } from '@/components/ui/Switch'
import { ClipService } from '@/gen/krill/v1/clip_pb'
import { PipelineService, type ParamValue } from '@/gen/krill/v1/pipeline_pb'
import { VideoService, VideoStatus } from '@/gen/krill/v1/video_pb'
import { errorMessage } from '@/lib/errors'

const DEBOUNCE = 400

function useDebounced<T>(value: T, ms: number): T {
  const [v, setV] = useState(value)
  useEffect(() => {
    const t = setTimeout(() => setV(value), ms)
    return () => clearTimeout(t)
  }, [value, ms])
  return v
}

// Picks a frame and shows what a plugin finds on it with the params being
// edited, so thresholds can be tuned by eye.
export function PrelabelPreview({
  name,
  params,
  colors,
}: {
  name: string
  params: ParamValue[]
  colors: Record<string, string>
}) {
  const [videoId, setVideoId] = useState(0n)
  const [clipId, setClipId] = useState(0n)
  const [index, setIndex] = useState(0)
  const [showMask, setShowMask] = useState(true)

  const videos = useQuery(VideoService.method.listVideos, {})
  const ready = (videos.data?.videos ?? []).filter((v) => v.status === VideoStatus.READY)
  const video = useQuery(VideoService.method.getVideo, { id: videoId }, { enabled: videoId > 0n })
  const clip = useQuery(ClipService.method.getClip, { id: clipId }, { enabled: clipId > 0n })
  const frames = clip.data?.frames ?? []
  const frame = frames[Math.min(index, frames.length - 1)]

  const request = useDebounced({ name, params, frameId: frame?.id ?? 0n }, DEBOUNCE)
  const preview = useQuery(PipelineService.method.previewPrelabeler, request, {
    enabled: request.frameId > 0n,
    placeholderData: keepPreviousData,
    retry: false,
    staleTime: Infinity,
  })
  const result = preview.data
  const current = result && frame && request.frameId === frame.id

  return (
    <div className="space-y-4">
      <div className="grid gap-4 sm:grid-cols-2">
        <Field>
          <Label>Video</Label>
          <Select
            value={String(videoId)}
            onChange={(e) => {
              setVideoId(BigInt(e.target.value))
              setClipId(0n)
              setIndex(0)
            }}
          >
            <option value="0">Choose…</option>
            {ready.map((v) => (
              <option key={String(v.id)} value={String(v.id)}>
                {v.name}
              </option>
            ))}
          </Select>
        </Field>
        <Field disabled={!video.data}>
          <Label>Clip</Label>
          <Select
            value={String(clipId)}
            onChange={(e) => {
              setClipId(BigInt(e.target.value))
              setIndex(0)
            }}
          >
            <option value="0">Choose…</option>
            {(video.data?.clips ?? []).map((c) => (
              <option key={String(c.id)} value={String(c.id)}>
                Clip {c.index + 1}
              </option>
            ))}
          </Select>
        </Field>
      </div>
      {frames.length > 0 && (
        <div className="flex items-center gap-4">
          <input
            type="range"
            aria-label="Frame"
            min={0}
            max={frames.length - 1}
            value={Math.min(index, frames.length - 1)}
            onChange={(e) => setIndex(Number(e.target.value))}
            className="flex-1 accent-sky-500"
          />
          <span className="w-24 text-right text-sm/6 text-zinc-500 tabular-nums dark:text-zinc-400">
            Frame {(frame?.index ?? 0) + 1}
          </span>
        </div>
      )}
      {frame && (
        <div className="relative overflow-hidden rounded-lg bg-zinc-950">
          <img src={frame.url} alt="" className="block w-full" crossOrigin="anonymous" />
          {showMask && current && result.maskUrl && (
            <div
              className="absolute inset-0 bg-sky-400/60"
              style={{
                maskImage: `url("${result.maskUrl}")`,
                maskSize: '100% 100%',
                maskMode: 'luminance',
              }}
            />
          )}
          {current && (
            <svg
              className="absolute inset-0 size-full"
              viewBox="0 0 1 1"
              preserveAspectRatio="none"
              aria-hidden="true"
            >
              {result.boxes.map((b, i) => {
                const color = colors[b.output] ?? '#facc15'
                return (
                  <g key={i}>
                    {b.box && (
                      <rect
                        x={b.box.x}
                        y={b.box.y}
                        width={b.box.width}
                        height={b.box.height}
                        fill="none"
                        stroke={color}
                        strokeWidth={2}
                        strokeDasharray={b.corners.length > 0 ? '4 3' : undefined}
                        vectorEffect="non-scaling-stroke"
                      />
                    )}
                    {b.corners.length > 0 && (
                      <polygon
                        points={b.corners.map((p) => `${p.x},${p.y}`).join(' ')}
                        fill="none"
                        stroke={color}
                        strokeWidth={2}
                        vectorEffect="non-scaling-stroke"
                      />
                    )}
                  </g>
                )
              })}
            </svg>
          )}
          {current &&
            result.boxes.map(
              (b, i) =>
                b.box && (
                  <span
                    key={i}
                    className="absolute -translate-y-full rounded-sm px-1 text-xs/5 font-medium text-zinc-950"
                    style={{
                      left: `${b.box.x * 100}%`,
                      top: `${b.box.y * 100}%`,
                      backgroundColor: colors[b.output] ?? '#facc15',
                    }}
                  >
                    {b.output} {Math.round(b.confidence * 100)}%
                  </span>
                ),
            )}
          {preview.isFetching && (
            <div className="absolute top-3 right-3">
              <LoadingSpinner />
            </div>
          )}
        </div>
      )}
      {frame && (
        <div className="flex flex-wrap items-center justify-between gap-4 text-sm/6 text-zinc-500 tabular-nums dark:text-zinc-400">
          <span>
            {current
              ? `${result.boxes.length} ${result.boxes.length === 1 ? 'box' : 'boxes'} · ${result.tookMs} ms`
              : '—'}
          </span>
          <span className="flex items-center gap-3">
            Mask
            <Switch aria-label="Mask" checked={showMask} onChange={setShowMask} />
          </span>
        </div>
      )}
      {preview.error && <Callout>{errorMessage(preview.error)}</Callout>}
    </div>
  )
}
