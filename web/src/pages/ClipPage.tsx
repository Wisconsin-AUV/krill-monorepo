import { useQuery } from '@connectrpc/connect-query'
import {
  ChevronLeftIcon,
  ChevronRightIcon,
  PauseIcon,
  PlayIcon,
  QuestionMarkCircleIcon,
} from '@heroicons/react/20/solid'
import { useEffect, useState } from 'react'
import { Link, useNavigate, useParams, useSearchParams } from 'react-router'
import { Button } from '@/components/ui/Button'
import LoadingSpinner from '@/components/ui/LoadingSpinner'
import { ClipService, type GetClipResponse } from '@/gen/krill/v1/clip_pb'
import { errorMessage } from '@/lib/errors'
import { FrameCanvas } from '@/workspace/FrameCanvas'
import { ShortcutsDialog } from '@/workspace/ShortcutsDialog'
import { Timeline } from '@/workspace/Timeline'
import { navigationShortcuts, viewShortcuts } from '@/workspace/shortcuts'
import { useHotkeys } from '@/workspace/useHotkeys'
import { useDarkDocument } from '@/workspace/useDarkDocument'
import { usePlayback } from '@/workspace/usePlayback'
import { fitView, useWorkspaceStore } from '@/workspace/useWorkspaceStore'

function parseId(raw: string | undefined): bigint | undefined {
  return raw && /^\d+$/.test(raw) ? BigInt(raw) : undefined
}

function useFrameParam(data: GetClipResponse | undefined) {
  const [params, setParams] = useSearchParams()
  const index = useWorkspaceStore((s) => s.index)
  const frames = useWorkspaceStore((s) => s.frames)
  const setFrames = useWorkspaceStore((s) => s.setFrames)
  const setImageSize = useWorkspaceStore((s) => s.setImageSize)

  useEffect(() => {
    if (!data) return
    const wanted = Number(new URLSearchParams(window.location.search).get('frame'))
    const start = data.frames.findIndex((f) => f.index === wanted)
    setFrames(data.frames, start >= 0 ? start : 0)
    setImageSize({ width: data.video?.width ?? 0, height: data.video?.height ?? 0 })
  }, [data, setFrames, setImageSize])

  const current = frames[index]?.index
  useEffect(() => {
    if (current === undefined || params.get('frame') === String(current)) return
    const t = setTimeout(() => setParams({ frame: String(current) }, { replace: true }), 250)
    return () => clearTimeout(t)
  }, [current, params, setParams])
}

export function ClipPage() {
  const id = parseId(useParams().id)
  const navigate = useNavigate()
  const [showHelp, setShowHelp] = useState(false)
  useDarkDocument()
  const { data, error, isPending } = useQuery(
    ClipService.method.getClip,
    { id },
    { enabled: id !== undefined },
  )
  useFrameParam(data)

  const index = useWorkspaceStore((s) => s.index)
  const frameCount = useWorkspaceStore((s) => s.frames.length)
  const playing = useWorkspaceStore((s) => s.playing)
  const view = useWorkspaceStore((s) => s.view)
  const fitScale = useWorkspaceStore((s) => fitView(s.stageSize, s.imageSize).scale)
  const step = useWorkspaceStore((s) => s.step)
  const seek = useWorkspaceStore((s) => s.seek)
  const setPlaying = useWorkspaceStore((s) => s.setPlaying)
  const fit = useWorkspaceStore((s) => s.fit)
  usePlayback(data?.video?.fps ?? 0)

  const goClip = (clipId: bigint | undefined) => {
    if (clipId) navigate(`/clips/${clipId}`)
  }

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
    End: () => seek(frameCount - 1),
    p: () => setPlaying(!playing),
    f: fit,
    '[': () => goClip(data?.previousClipId),
    ']': () => goClip(data?.nextClipId),
    '?': () => setShowHelp(true),
  })

  useEffect(() => {
    if (data?.video)
      document.title = `${data.video.name} · Clip ${(data.clip?.index ?? 0) + 1} · Krill`
  }, [data])

  if (id === undefined || error) {
    return (
      <div className="dark flex h-dvh flex-col items-center justify-center gap-4 bg-zinc-950 text-zinc-300">
        <p>{error ? `Could not load clip: ${errorMessage(error)}` : 'Clip not found.'}</p>
        <Button to="/" outline>
          Back to videos
        </Button>
      </div>
    )
  }
  if (isPending || !data.video || !data.clip) {
    return (
      <div className="dark flex h-dvh items-center justify-center bg-zinc-950">
        <LoadingSpinner />
      </div>
    )
  }

  const { video, clip } = data
  const zoomPct = fitScale > 0 ? Math.round((view.scale / fitScale) * 100) : 100

  return (
    <div className="dark flex h-dvh flex-col bg-zinc-950 text-zinc-100">
      <header className="flex h-12 shrink-0 items-center gap-2 border-b border-white/10 bg-zinc-900 px-2">
        <Link
          to={`/videos/${video.id}`}
          className="flex min-w-0 items-center gap-1 rounded-md px-2 py-1 text-sm/6 text-zinc-400 hover:bg-white/5 hover:text-white"
        >
          <ChevronLeftIcon className="size-4 shrink-0" />
          <span className="truncate">{video.name}</span>
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

        <div className="flex-1" />

        <span className="px-2 text-sm/6 text-zinc-400 tabular-nums">
          Frame <span className="text-zinc-100">{index + 1}</span> / {frameCount}
        </span>
        <Button plain aria-label={playing ? 'Pause' : 'Play'} onClick={() => setPlaying(!playing)}>
          {playing ? <PauseIcon data-slot="icon" /> : <PlayIcon data-slot="icon" />}
        </Button>
        <Button plain aria-label="Fit to screen" title="Fit to screen (F)" onClick={fit}>
          <span className="w-9 text-center text-xs/6 tabular-nums">{zoomPct}%</span>
        </Button>
        <Button plain aria-label="Keyboard shortcuts" onClick={() => setShowHelp(true)}>
          <QuestionMarkCircleIcon data-slot="icon" />
        </Button>
      </header>

      <div className="flex min-h-0 flex-1">
        <main className="relative min-w-0 flex-1 bg-[radial-gradient(var(--color-zinc-800)_1px,transparent_1px)] [background-size:16px_16px]">
          <FrameCanvas />
        </main>
      </div>

      <Timeline />
      <ShortcutsDialog
        open={showHelp}
        onClose={() => setShowHelp(false)}
        groups={[navigationShortcuts, viewShortcuts]}
      />
    </div>
  )
}
