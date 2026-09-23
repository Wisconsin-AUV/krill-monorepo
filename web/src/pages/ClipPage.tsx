import { useQuery } from '@connectrpc/connect-query'
import {
  CheckIcon,
  ChevronLeftIcon,
  ChevronRightIcon,
  EyeIcon,
  EyeSlashIcon,
  NoSymbolIcon,
  PauseIcon,
  PlayIcon,
  QuestionMarkCircleIcon,
} from '@heroicons/react/20/solid'
import { useEffect, useMemo, useState } from 'react'
import { Link, useNavigate, useParams, useSearchParams } from 'react-router'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import LoadingSpinner from '@/components/ui/LoadingSpinner'
import { FrameStatus } from '@/gen/krill/v1/annotation_pb'
import { ClipService, type GetClipResponse } from '@/gen/krill/v1/clip_pb'
import { LabelService } from '@/gen/krill/v1/label_pb'
import { VideoService } from '@/gen/krill/v1/video_pb'
import { errorMessage } from '@/lib/errors'
import { flash } from '@/lib/flash'
import { invalidateService } from '@/lib/queryClient'
import type { TrackInfo } from '@/labeling/AnnotationLayer'
import { LabelCanvas } from '@/labeling/LabelCanvas'
import { TrackPanel } from '@/labeling/TrackPanel'
import { TypePanel } from '@/labeling/TypePanel'
import { frameState } from '@/labeling/frameStatus'
import { frameShortcuts, labelingShortcuts } from '@/labeling/shortcuts'
import { idKey, useLabelStore } from '@/labeling/useLabelStore'
import { ShortcutsDialog } from '@/workspace/ShortcutsDialog'
import { Timeline } from '@/workspace/Timeline'
import { navigationShortcuts, viewShortcuts } from '@/workspace/shortcuts'
import { useDarkDocument } from '@/workspace/useDarkDocument'
import { useHotkeys } from '@/workspace/useHotkeys'
import { usePlayback } from '@/workspace/usePlayback'
import { fitView, useWorkspaceStore } from '@/workspace/useWorkspaceStore'

function parseId(raw: string | undefined): bigint | undefined {
  return raw && /^\d+$/.test(raw) ? BigInt(raw) : undefined
}

function useClipState(data: GetClipResponse | undefined) {
  const [params, setParams] = useSearchParams()
  const index = useWorkspaceStore((s) => s.index)
  const frames = useWorkspaceStore((s) => s.frames)
  const setFrames = useWorkspaceStore((s) => s.setFrames)
  const setImageSize = useWorkspaceStore((s) => s.setImageSize)
  const load = useLabelStore((s) => s.load)

  useEffect(() => {
    if (!data) return
    const wanted = Number(new URLSearchParams(window.location.search).get('frame'))
    const start = data.frames.findIndex((f) => f.index === wanted)
    setFrames(data.frames, start >= 0 ? start : 0)
    setImageSize({ width: data.video?.width ?? 0, height: data.video?.height ?? 0 })
    load(data)
  }, [data, setFrames, setImageSize, load])

  const current = frames[index]?.index
  useEffect(() => {
    if (current === undefined || params.get('frame') === String(current)) return
    const t = setTimeout(() => setParams({ frame: String(current) }, { replace: true }), 250)
    return () => clearTimeout(t)
  }, [current, params, setParams])

  useEffect(
    () => () => {
      void invalidateService(VideoService)
      void invalidateService(LabelService)
    },
    [],
  )
}

function SaveIndicator() {
  const pending = useLabelStore((s) => s.pending)
  return (
    <span className="w-14 text-xs text-zinc-500" aria-live="polite">
      {pending > 0 ? 'Saving…' : 'Saved'}
    </span>
  )
}

export function ClipPage() {
  const id = parseId(useParams().id)
  const navigate = useNavigate()
  const [showHelp, setShowHelp] = useState(false)
  useDarkDocument()

  const { data, error, isPending } = useQuery(
    ClipService.method.getClip,
    { id },
    { enabled: id !== undefined, staleTime: Infinity },
  )
  const { data: typeData } = useQuery(LabelService.method.listLabelTypes, {})
  useClipState(data)
  usePlayback(data?.video?.fps ?? 0)

  const types = useMemo(() => typeData?.labelTypes ?? [], [typeData])
  const frames = useWorkspaceStore((s) => s.frames)
  const index = useWorkspaceStore((s) => s.index)
  const playing = useWorkspaceStore((s) => s.playing)
  const view = useWorkspaceStore((s) => s.view)
  const fitScale = useWorkspaceStore((s) => fitView(s.stageSize, s.imageSize).scale)
  const step = useWorkspaceStore((s) => s.step)
  const seek = useWorkspaceStore((s) => s.seek)
  const setPlaying = useWorkspaceStore((s) => s.setPlaying)
  const fit = useWorkspaceStore((s) => s.fit)

  const trackMap = useLabelStore((s) => s.tracks)
  const boxes = useLabelStore((s) => s.boxes)
  const frameStatus = useLabelStore((s) => s.frameStatus)
  const selectedTrackId = useLabelStore((s) => s.selectedTrackId)
  const hideBoxes = useLabelStore((s) => s.hideBoxes)
  const labels = useLabelStore.getState

  useEffect(() => {
    const active = labels().activeTypeId
    if (types.length > 0 && (active === null || !types.some((t) => t.id === active))) {
      labels().setActiveType(types[0].id)
    }
  }, [types, labels])

  const tracks = useMemo(() => {
    const byId = new Map(types.map((t) => [t.id, t]))
    const out = new Map<string, TrackInfo>()
    Object.values(trackMap)
      .sort((a, b) => (a.id < b.id ? -1 : 1))
      .forEach((track, i) =>
        out.set(idKey(track.id), { number: i + 1, track, type: byId.get(track.labelTypeId) }),
      )
    return out
  }, [trackMap, types])

  const typeCounts = useMemo(() => {
    const counts = new Map<bigint, number>()
    for (const t of Object.values(trackMap))
      counts.set(t.labelTypeId, (counts.get(t.labelTypeId) ?? 0) + 1)
    return counts
  }, [trackMap])

  const frame = frames[index]
  const frameBoxes = frame ? Object.values(boxes[idKey(frame.id)] ?? {}) : []
  const status = frame ? frameStatus[idKey(frame.id)] : undefined
  const state = frameState(status, frameBoxes.length)
  const doneCount = frames.filter((f) => {
    const st = frameStatus[idKey(f.id)]
    return st === FrameStatus.LABELED || st === FrameStatus.EMPTY
  }).length

  const goClip = (clipId: bigint | undefined) => {
    if (clipId) navigate(`/clips/${clipId}`)
  }

  async function mark(next: FrameStatus, advance: boolean) {
    if (!frame) return
    if (next === FrameStatus.EMPTY && frameBoxes.length > 0) {
      flash.error('Frame has boxes', 'Delete them before marking the frame empty.')
      return
    }
    const ok = await labels().setFrameStatus(frame.id, next)
    if (ok && advance) {
      if (index < frames.length - 1) step(1)
      else if (data?.nextClipId)
        flash.success('Last frame of the clip', 'Press ] to go to the next clip.')
    }
  }

  function cycleSelection(delta: number) {
    if (frameBoxes.length === 0) return
    const ids = frameBoxes.map((b) => b.trackId).sort((a, b) => (a < b ? -1 : 1))
    const at = ids.findIndex((t) => t === selectedTrackId)
    labels().select(ids[(at + delta + ids.length) % ids.length])
  }

  const typeKeys = Object.fromEntries(
    types.slice(0, 9).map((t, i) => [String(i + 1), () => labels().setActiveType(t.id)]),
  )

  useHotkeys({
    j: () => step(-1),
    k: () => step(1),
    ArrowLeft: () => step(-1),
    ArrowRight: () => step(1),
    J: () => step(-10),
    K: () => step(10),
    'shift+ArrowLeft': () => step(-10),
    'shift+ArrowRight': () => step(10),
    Home: () => seek(0),
    End: () => seek(frames.length - 1),
    p: () => setPlaying(!playing),
    f: fit,
    '[': () => goClip(data?.previousClipId),
    ']': () => goClip(data?.nextClipId),
    '?': () => setShowHelp(true),
    ...typeKeys,
    Escape: () => labels().select(null),
    Tab: () => cycleSelection(1),
    'shift+Tab': () => cycleSelection(-1),
    Delete: () =>
      frame && selectedTrackId !== null && void labels().deleteBox(selectedTrackId, frame.id),
    Backspace: () =>
      frame && selectedTrackId !== null && void labels().deleteBox(selectedTrackId, frame.id),
    'shift+Delete': () => selectedTrackId !== null && void labels().deleteTrack(selectedTrackId),
    'shift+Backspace': () => selectedTrackId !== null && void labels().deleteTrack(selectedTrackId),
    c: async () => {
      if (!frame || index === 0) return
      const n = await labels().copyBoxes(frames[index - 1].id, frame.id)
      if (n === 0)
        flash.error('Nothing to copy', 'Every box on the previous frame is already here.')
    },
    h: () => labels().toggleHidden(),
    ' ': () => void mark(FrameStatus.LABELED, true),
    e: () => void mark(FrameStatus.EMPTY, true),
    u: () => void mark(FrameStatus.UNLABELED, false),
    'mod+z': () => void labels().undo(),
  })

  useEffect(() => {
    if (data?.video)
      document.title = `${data.video.name} · Clip ${(data.clip?.index ?? 0) + 1} · Krill`
  }, [data])

  if (id === undefined || error) {
    return (
      <div className="flex h-dvh flex-col items-center justify-center gap-4 bg-zinc-950 text-zinc-300">
        <p>{error ? `Could not load clip: ${errorMessage(error)}` : 'Clip not found.'}</p>
        <Button to="/" outline>
          Back to videos
        </Button>
      </div>
    )
  }
  if (isPending || !data.video || !data.clip) {
    return (
      <div className="flex h-dvh items-center justify-center bg-zinc-950">
        <LoadingSpinner />
      </div>
    )
  }

  const { video, clip } = data
  const zoomPct = fitScale > 0 ? Math.round((view.scale / fitScale) * 100) : 100
  const selectedKey = selectedTrackId !== null ? idKey(selectedTrackId) : null

  return (
    <div className="flex h-dvh flex-col bg-zinc-950 text-zinc-100">
      <header className="flex h-12 shrink-0 items-center gap-2 border-b border-white/10 bg-zinc-900 px-2">
        <Link
          to={`/videos/${video.id}`}
          className="flex min-w-0 items-center gap-1 rounded-md px-2 py-1 text-sm/6 text-zinc-400 hover:bg-white/5 hover:text-white"
        >
          <ChevronLeftIcon className="size-4 shrink-0" />
          <span className="max-w-48 truncate">{video.name}</span>
        </Link>
        <div className="mx-1 h-5 w-px bg-white/10" />
        <div className="flex items-center">
          <Button
            plain
            aria-label="Previous clip"
            disabled={!data.previousClipId}
            onClick={() => goClip(data.previousClipId)}
          >
            <ChevronLeftIcon data-slot="icon" />
          </Button>
          <span className="px-1 text-sm/6 font-medium whitespace-nowrap tabular-nums">
            Clip {clip.index + 1} <span className="text-zinc-500">of {video.clipCount}</span>
          </span>
          <Button
            plain
            aria-label="Next clip"
            disabled={!data.nextClipId}
            onClick={() => goClip(data.nextClipId)}
          >
            <ChevronRightIcon data-slot="icon" />
          </Button>
        </div>
        <span className="ml-2 text-xs text-zinc-500 tabular-nums">
          {doneCount}/{frames.length} done
        </span>

        <div className="flex-1" />

        <SaveIndicator />
        <Badge color={state.color}>{state.label}</Badge>
        <Button
          plain
          title="Mark empty (E)"
          disabled={frameBoxes.length > 0 || status === FrameStatus.EMPTY}
          onClick={() => void mark(FrameStatus.EMPTY, true)}
        >
          <NoSymbolIcon data-slot="icon" />
          Empty
        </Button>
        <Button
          color="emerald"
          title="Mark labeled and continue (Space)"
          onClick={() => void mark(FrameStatus.LABELED, true)}
        >
          <CheckIcon data-slot="icon" />
          Done
        </Button>
        <div className="mx-1 h-5 w-px bg-white/10" />
        <span className="px-1 text-sm/6 text-zinc-400 tabular-nums">
          Frame <span className="text-zinc-100">{index + 1}</span> / {frames.length}
        </span>
        <Button plain aria-label={playing ? 'Pause' : 'Play'} onClick={() => setPlaying(!playing)}>
          {playing ? <PauseIcon data-slot="icon" /> : <PlayIcon data-slot="icon" />}
        </Button>
        <Button
          plain
          aria-label={hideBoxes ? 'Show boxes' : 'Hide boxes'}
          title="Hide boxes (H)"
          onClick={() => labels().toggleHidden()}
        >
          {hideBoxes ? <EyeSlashIcon data-slot="icon" /> : <EyeIcon data-slot="icon" />}
        </Button>
        <Button plain aria-label="Fit to screen" title="Fit to screen (F)" onClick={fit}>
          <span className="w-9 text-center text-xs/6 tabular-nums">{zoomPct}%</span>
        </Button>
        <Button plain aria-label="Keyboard shortcuts" onClick={() => setShowHelp(true)}>
          <QuestionMarkCircleIcon data-slot="icon" />
        </Button>
      </header>

      <div className="flex min-h-0 flex-1">
        <TypePanel types={types} counts={typeCounts} />
        <main className="relative min-w-0 flex-1 bg-[radial-gradient(var(--color-zinc-800)_1px,transparent_1px)] [background-size:16px_16px]">
          <LabelCanvas tracks={tracks} types={types} />
        </main>
        <TrackPanel tracks={tracks} types={types} />
      </div>

      <Timeline
        frameClass={(i) => {
          const f = frames[i]
          const byTrack = boxes[idKey(f.id)]
          return frameState(frameStatus[idKey(f.id)], byTrack ? Object.keys(byTrack).length : 0).bar
        }}
        trackClass={
          selectedKey
            ? (i) => (boxes[idKey(frames[i].id)]?.[selectedKey] ? 'bg-sky-400' : undefined)
            : undefined
        }
      />
      <ShortcutsDialog
        open={showHelp}
        onClose={() => setShowHelp(false)}
        groups={[labelingShortcuts, frameShortcuts, navigationShortcuts, viewShortcuts]}
      />
    </div>
  )
}
