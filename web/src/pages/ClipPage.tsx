import { useQuery } from '@connectrpc/connect-query'
import { keepPreviousData } from '@tanstack/react-query'
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
import { useNavigate, useParams, useSearchParams } from 'react-router'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import LoadingSpinner from '@/components/ui/LoadingSpinner'
import {
  Navbar,
  NavbarDivider,
  NavbarItem,
  NavbarLabel,
  NavbarSection,
  NavbarSpacer,
} from '@/components/ui/Navbar'
import {
  Sidebar,
  SidebarBody,
  SidebarItem,
  SidebarLabel,
  SidebarSection,
} from '@/components/ui/Sidebar'
import { StackedLayout } from '@/components/ui/StackedLayout'
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
import { useHotkeys } from '@/workspace/useHotkeys'
import { usePlayback } from '@/workspace/usePlayback'
import { usePrefetchNeighbours } from '@/workspace/usePrefetchNeighbours'
import { fitView, useWorkspaceStore } from '@/workspace/useWorkspaceStore'

function parseId(raw: string | undefined): bigint | undefined {
  return raw && /^\d+$/.test(raw) ? BigInt(raw) : undefined
}

function useClipState(data: GetClipResponse | undefined, switching: boolean) {
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
    if (switching || current === undefined || params.get('frame') === String(current)) return
    const t = setTimeout(() => setParams({ frame: String(current) }, { replace: true }), 250)
    return () => clearTimeout(t)
  }, [current, params, setParams, switching])

  useEffect(
    () => () => {
      void invalidateService(VideoService)
      void invalidateService(LabelService)
    },
    [],
  )
}

function BackItem({ to, label }: { to: string; label: string }) {
  return (
    <NavbarItem to={to} current={false} className="max-lg:hidden">
      <ChevronLeftIcon data-slot="icon" />
      <NavbarLabel className="max-w-48">{label}</NavbarLabel>
    </NavbarItem>
  )
}

// The tagger replaces the app navigation with its own toolbar, so it has a
// layout of its own instead of living under AppLayout.
function ClipShell({
  navbar,
  sidebar,
  children,
}: {
  navbar: React.ReactNode
  sidebar?: React.ReactNode
  children: React.ReactNode
}) {
  return (
    <StackedLayout fill navbar={navbar} sidebar={sidebar ?? <Sidebar />}>
      {children}
    </StackedLayout>
  )
}

function SaveIndicator() {
  const pending = useLabelStore((s) => s.pending)
  return (
    <span className="w-14 text-xs text-zinc-500 dark:text-zinc-400" aria-live="polite">
      {pending > 0 ? 'Saving…' : 'Saved'}
    </span>
  )
}

export function ClipPage() {
  const id = parseId(useParams().id)
  const navigate = useNavigate()
  const [showHelp, setShowHelp] = useState(false)

  // Labels live in the store while a clip is open, so the query must not
  // refetch underneath it. gcTime 0 drops a clip once it is left, so coming
  // back always loads fresh labels. The previous clip stays on screen while
  // the next one loads so the layout never flashes a spinner.
  const { data, error, isPending, isPlaceholderData } = useQuery(
    ClipService.method.getClip,
    { id },
    {
      enabled: id !== undefined,
      staleTime: Infinity,
      gcTime: 0,
      placeholderData: keepPreviousData,
    },
  )
  const switching = isPlaceholderData
  usePrefetchNeighbours(switching ? undefined : data)
  const { data: typeData } = useQuery(LabelService.method.listLabelTypes, {})
  useClipState(data, switching)
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

  useHotkeys(
    switching
      ? {}
      : {
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
          'shift+Delete': () =>
            selectedTrackId !== null && void labels().deleteTrack(selectedTrackId),
          'shift+Backspace': () =>
            selectedTrackId !== null && void labels().deleteTrack(selectedTrackId),
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
        },
  )

  useEffect(() => {
    if (data?.video)
      document.title = `${data.video.name} · Clip ${(data.clip?.index ?? 0) + 1} · Krill`
  }, [data])

  if (id === undefined || error) {
    return (
      <ClipShell navbar={<BackItem to="/" label="Home" />}>
        <div className="flex flex-1 flex-col items-center justify-center gap-4 py-24 text-zinc-700 dark:text-zinc-300">
          <p>{error ? `Could not load clip: ${errorMessage(error)}` : 'Clip not found.'}</p>
          <Button to="/" outline>
            Back home
          </Button>
        </div>
      </ClipShell>
    )
  }
  if (isPending || !data.video || !data.clip) {
    return (
      <ClipShell navbar={<BackItem to="/" label="Home" />}>
        <div className="flex flex-1 items-center justify-center py-24">
          <LoadingSpinner />
        </div>
      </ClipShell>
    )
  }

  const { video, clip } = data
  const zoomPct = fitScale > 0 ? Math.round((view.scale / fitScale) * 100) : 100
  const selectedKey = selectedTrackId !== null ? idKey(selectedTrackId) : null

  const navbar = (
    <Navbar>
      <BackItem to={`/videos/${video.id}`} label={video.name} />
      <NavbarDivider className="max-lg:hidden" />
      <NavbarSection>
        <NavbarItem
          aria-label="Previous clip"
          disabled={!data.previousClipId}
          onClick={() => goClip(data.previousClipId)}
        >
          <ChevronLeftIcon data-slot="icon" />
        </NavbarItem>
        <span className="text-sm/6 font-medium whitespace-nowrap tabular-nums">
          Clip {clip.index + 1}{' '}
          <span className="text-zinc-500 dark:text-zinc-400">of {video.clipCount}</span>
        </span>
        <NavbarItem
          aria-label="Next clip"
          disabled={!data.nextClipId}
          onClick={() => goClip(data.nextClipId)}
        >
          <ChevronRightIcon data-slot="icon" />
        </NavbarItem>
        <span className="text-xs text-zinc-500 tabular-nums max-md:hidden dark:text-zinc-400">
          {doneCount}/{frames.length} done
        </span>
      </NavbarSection>
      <NavbarSpacer />
      <NavbarSection>
        <SaveIndicator />
        <Badge color={state.color}>{state.label}</Badge>
        <Button
          plain
          title="Mark empty (E)"
          disabled={switching || frameBoxes.length > 0 || status === FrameStatus.EMPTY}
          onClick={() => void mark(FrameStatus.EMPTY, true)}
        >
          <NoSymbolIcon data-slot="icon" />
          Empty
        </Button>
        <Button
          color="emerald"
          disabled={switching}
          title="Mark labeled and continue (Space)"
          onClick={() => void mark(FrameStatus.LABELED, true)}
        >
          <CheckIcon data-slot="icon" />
          Done
        </Button>
      </NavbarSection>
      <NavbarDivider className="max-lg:hidden" />
      <NavbarSection className="max-lg:hidden">
        <span className="text-sm/6 text-zinc-500 tabular-nums dark:text-zinc-400">
          Frame <span className="text-zinc-950 dark:text-white">{index + 1}</span> / {frames.length}
        </span>
        <NavbarItem aria-label={playing ? 'Pause' : 'Play'} onClick={() => setPlaying(!playing)}>
          {playing ? <PauseIcon data-slot="icon" /> : <PlayIcon data-slot="icon" />}
        </NavbarItem>
        <NavbarItem
          aria-label={hideBoxes ? 'Show boxes' : 'Hide boxes'}
          title="Hide boxes (H)"
          onClick={() => labels().toggleHidden()}
        >
          {hideBoxes ? <EyeSlashIcon data-slot="icon" /> : <EyeIcon data-slot="icon" />}
        </NavbarItem>
        <NavbarItem aria-label="Fit to screen" title="Fit to screen (F)" onClick={fit}>
          <span className="min-w-10 text-center tabular-nums">{zoomPct}%</span>
        </NavbarItem>
        <NavbarItem aria-label="Keyboard shortcuts" onClick={() => setShowHelp(true)}>
          <QuestionMarkCircleIcon data-slot="icon" />
        </NavbarItem>
      </NavbarSection>
    </Navbar>
  )

  const sidebar = (
    <Sidebar>
      <SidebarBody>
        <SidebarSection>
          <SidebarItem to={`/videos/${video.id}`}>
            <ChevronLeftIcon data-slot="icon" />
            <SidebarLabel>{video.name}</SidebarLabel>
          </SidebarItem>
          <SidebarItem disabled={!data.previousClipId} onClick={() => goClip(data.previousClipId)}>
            <SidebarLabel>Previous clip</SidebarLabel>
          </SidebarItem>
          <SidebarItem disabled={!data.nextClipId} onClick={() => goClip(data.nextClipId)}>
            <SidebarLabel>Next clip</SidebarLabel>
          </SidebarItem>
          <SidebarItem onClick={() => setShowHelp(true)}>
            <QuestionMarkCircleIcon data-slot="icon" />
            <SidebarLabel>Keyboard shortcuts</SidebarLabel>
          </SidebarItem>
        </SidebarSection>
      </SidebarBody>
    </Sidebar>
  )

  return (
    <ClipShell navbar={navbar} sidebar={sidebar}>
      <div className="flex min-h-0 flex-1 flex-col text-zinc-950 dark:text-zinc-100">
        <div className="flex min-h-0 flex-1">
          <TypePanel types={types} counts={typeCounts} />
          <main className="relative min-w-0 flex-1 bg-zinc-950 bg-[radial-gradient(var(--color-zinc-800)_1px,transparent_1px)] [background-size:16px_16px]">
            <LabelCanvas tracks={tracks} types={types} />
            {switching && (
              <div className="absolute inset-0 flex items-center justify-center bg-zinc-950/40">
                <LoadingSpinner />
              </div>
            )}
          </main>
          <TrackPanel tracks={tracks} types={types} />
        </div>

        <Timeline
          frameClass={(i) => {
            const f = frames[i]
            const byTrack = boxes[idKey(f.id)]
            return frameState(frameStatus[idKey(f.id)], byTrack ? Object.keys(byTrack).length : 0)
              .bar
          }}
          trackClass={
            selectedKey
              ? (i) =>
                  boxes[idKey(frames[i].id)]?.[selectedKey]
                    ? 'bg-sky-500 dark:bg-sky-400'
                    : undefined
              : undefined
          }
        />
        <ShortcutsDialog
          open={showHelp}
          onClose={() => setShowHelp(false)}
          groups={[labelingShortcuts, frameShortcuts, navigationShortcuts, viewShortcuts]}
        />
      </div>
    </ClipShell>
  )
}
