import { useMutation, useQuery } from '@connectrpc/connect-query'
import { keepPreviousData } from '@tanstack/react-query'
import {
  BeakerIcon,
  CheckIcon,
  ChevronLeftIcon,
  ChevronRightIcon,
  ForwardIcon,
  EyeIcon,
  EyeSlashIcon,
  FlagIcon,
  NoSymbolIcon,
  QuestionMarkCircleIcon,
  SparklesIcon,
  StarIcon,
  TrashIcon,
} from '@heroicons/react/20/solid'
import { useEffect, useMemo, useState } from 'react'
import { Link, useNavigate, useParams, useSearchParams } from 'react-router'
import { ConfirmAlert } from '@/components/ConfirmAlert'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { Dropdown, DropdownButton, DropdownItem, DropdownMenu } from '@/components/ui/Dropdown'
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
import { ClipService, type Frame, type GetClipResponse } from '@/gen/krill/v1/clip_pb'
import { GoldService } from '@/gen/krill/v1/gold_pb'
import { LabelService } from '@/gen/krill/v1/label_pb'
import { QueueService } from '@/gen/krill/v1/queue_pb'
import {
  ReviewReason,
  ReviewService,
  type Review,
  type ReviewResolution,
} from '@/gen/krill/v1/review_pb'
import { Permission } from '@/gen/krill/v1/user_pb'
import { VideoService } from '@/gen/krill/v1/video_pb'
import { useCan, useUser } from '@/lib/auth'
import { contentClient } from '@/lib/clients'
import { errorMessage } from '@/lib/errors'
import { flash } from '@/lib/flash'
import { plural } from '@/lib/format'
import { invalidateService } from '@/lib/queryClient'
import { takenByOther, useClaimNextClip } from '@/lib/queue'
import { useUIStore } from '@/store/useUIStore'
import { reasons, resolutions } from '@/lib/reviews'
import type { TrackInfo } from '@/labeling/AnnotationLayer'
import { LabelCanvas } from '@/labeling/LabelCanvas'
import { TrackPanel } from '@/labeling/TrackPanel'
import { TrialLayer } from '@/labeling/TrialLayer'
import { TrialPanel } from '@/labeling/TrialPanel'
import { TypePanel } from '@/labeling/TypePanel'
import { frameState } from '@/labeling/frameStatus'
import { reviewTracks } from '@/labeling/review'
import {
  frameShortcuts,
  goldShortcut,
  labelingShortcuts,
  trialShortcuts,
} from '@/labeling/shortcuts'
import {
  isDone,
  nextDisagreement,
  reviewTrial,
  verifiedBoxes,
  type Verdict,
} from '@/labeling/trialMatch'
import { idKey, useLabelStore } from '@/labeling/useLabelStore'
import { useTrackingPoll } from '@/labeling/useTrackingPoll'
import { useTrialPoll } from '@/labeling/useTrialPoll'
import { useTrialStore } from '@/labeling/useTrialStore'
import { ShortcutsDialog } from '@/workspace/ShortcutsDialog'
import { Timeline } from '@/workspace/Timeline'
import { navigationShortcuts, viewShortcuts } from '@/workspace/shortcuts'
import { useHotkeys } from '@/workspace/useHotkeys'
import { usePlayback } from '@/workspace/usePlayback'
import { usePrefetchNeighbours } from '@/workspace/usePrefetchNeighbours'
import { fitView, useWorkspaceStore } from '@/workspace/useWorkspaceStore'

function parseId(raw: string | null | undefined): bigint | undefined {
  return raw && /^\d+$/.test(raw) ? BigInt(raw) : undefined
}

const verdictBars: Record<Verdict, string> = {
  correct: 'bg-emerald-500',
  off: 'bg-amber-400',
  wrong: 'bg-rose-500',
  unchecked: 'bg-zinc-400/40 dark:bg-zinc-500/40',
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
    const t = setTimeout(
      () =>
        setParams(
          (prev) => {
            const next = new URLSearchParams(prev)
            next.set('frame', String(current))
            return next
          },
          { replace: true },
        ),
      250,
    )
    return () => clearTimeout(t)
  }, [current, params, setParams, switching])

  useEffect(
    () => () => {
      void invalidateService(VideoService)
      void invalidateService(LabelService)
      void invalidateService(QueueService)
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
  const me = useUser()
  const claimNext = useClaimNextClip()
  const [showHelp, setShowHelp] = useState(false)
  const [params, setParams] = useSearchParams()
  const trialId = parseId(params.get('trial'))
  const [showTrials, setShowTrials] = useState(trialId !== undefined)

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
  useTrackingPoll(switching ? undefined : data?.clip?.id)
  useTrialPoll(trialId, switching ? undefined : data?.clip?.id)
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
  const trial = useTrialStore((s) => s.trial)
  const trialBoxes = useTrialStore((s) => s.boxes)
  const threshold = useTrialStore((s) => s.threshold)
  const trialVisible = useTrialStore((s) => s.visible)
  const corrected = useUIStore((s) => s.displayCorrection)
  const toggleCorrected = useUIStore((s) => s.toggleDisplayCorrection)

  const canReview = useCan(Permission.REVIEW_LABELS)
  const [takenClipId, setTakenClipId] = useState<bigint>()
  const takeClip = useMutation(QueueService.method.takeClip, {
    onSuccess: (_, { clipId }) => setTakenClipId(clipId),
  })
  const canDelete = useCan(Permission.MANAGE_VIDEOS)
  const [deleting, setDeleting] = useState(false)
  const deleteClip = useMutation(ClipService.method.deleteClip, {
    onSuccess: (_, { id: trashed }) => {
      setDeleting(false)
      if (trashed !== undefined) {
        flash.undoable('Clip moved to trash', () => {
          contentClient
            .restoreTrash({ clipIds: [trashed] })
            .then(() =>
              Promise.all([
                invalidateService(ClipService),
                invalidateService(VideoService),
                invalidateService(QueueService),
              ]),
            )
            .then(() => navigate(`/clips/${trashed}`))
            .catch((err: unknown) => flash.error('Could not restore clip', err))
        })
      }
      const next = data?.nextClipId || data?.previousClipId
      navigate(next ? `/clips/${next}` : `/videos/${data?.video?.id}`, { replace: true })
      void invalidateService(ClipService)
      void invalidateService(VideoService)
      void invalidateService(QueueService)
    },
  })
  const [goldEdits, setGoldEdits] = useState<Record<string, boolean>>({})
  const setGoldFrame = useMutation(GoldService.method.setGoldFrame, {
    onError: (err) => flash.error('Could not update gold frame', err),
  })
  const [reviewEdits, setReviewEdits] = useState<Record<string, Review | null>>({})
  const flagFrame = useMutation(ReviewService.method.flagFrame, {
    onError: (err) => flash.error('Could not flag frame', err),
  })
  const resolveReview = useMutation(ReviewService.method.resolveReview, {
    onError: (err) => flash.error('Could not resolve review', err),
  })

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

  const review = useMemo(() => reviewTracks(frames, boxes), [frames, boxes])

  const frameBars = useMemo(
    () =>
      frames.map((f) => {
        const byTrack = boxes[idKey(f.id)]
        return frameState(frameStatus[idKey(f.id)], byTrack ? Object.keys(byTrack).length : 0).bar
      }),
    [frames, boxes, frameStatus],
  )
  const selectedBars = useMemo(() => {
    if (selectedTrackId === null) return undefined
    const k = idKey(selectedTrackId)
    return frames.map((f) => (boxes[idKey(f.id)]?.[k] ? 'bg-sky-500 dark:bg-sky-400' : undefined))
  }, [frames, boxes, selectedTrackId])
  const driftFlags = useMemo(
    () => frames.map((f) => review.drift.has(idKey(f.id))),
    [frames, review],
  )
  const reviewMarks = useMemo(
    () =>
      frames.map((f) => {
        const k = idKey(f.id)
        return !!(k in reviewEdits ? reviewEdits[k] : f.review)
      }),
    [frames, reviewEdits],
  )

  const processed = trial?.framesDone
  const trialReview = useMemo(
    () =>
      processed === undefined
        ? undefined
        : reviewTrial(frames, processed, trialBoxes, threshold, (frameId) => {
            const k = idKey(frameId)
            return isDone(frameStatus[k]) ? verifiedBoxes(boxes[k], trackMap) : undefined
          }),
    [processed, frames, trialBoxes, threshold, frameStatus, boxes, trackMap],
  )
  const trialBars = useMemo(
    () => trialReview?.verdicts.map((v) => (v ? verdictBars[v] : undefined)),
    [trialReview],
  )

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
  const finished = frames.length > 0 && doneCount === frames.length
  const boxCount = Object.values(boxes).reduce((n, byTrack) => n + Object.keys(byTrack).length, 0)

  // Releasing the finished clip keeps it out of the caller's open clips.
  const nextFromQueue = () => {
    if (finished && data?.clip && !claimNext.isPending)
      claimNext.mutate({ releaseClipId: data.clip.id })
  }

  const goClip = (clipId: bigint | undefined) => {
    if (clipId) navigate(`/clips/${clipId}`)
  }

  // Reads the stores directly because key repeat can fire again before React
  // re-renders, and a stale frame would get marked twice while the one after
  // it was skipped. A held key only marks frames that have been drawn, so it
  // can never verify a frame the labeler has not seen.
  function mark(next: FrameStatus, advance: boolean, e?: KeyboardEvent) {
    const live = useWorkspaceStore.getState()
    const f = live.frames[live.index]
    if (!f || (e?.repeat && f.id !== live.drawnFrameId)) return
    if (next === FrameStatus.EMPTY && Object.keys(labels().boxes[idKey(f.id)] ?? {}).length > 0) {
      if (!e?.repeat) flash.info('Frame has boxes', 'Delete them before marking the frame empty.')
      return
    }
    void labels().setFrameStatus(f.id, next)
    if (!advance) return
    if (live.index < live.frames.length - 1) step(1)
    else if (!e?.repeat) flash.info('Last frame of the clip')
  }

  const isGold = !!frame && (goldEdits[idKey(frame.id)] ?? frame.gold)
  const canGold = canReview && !!frame && (isGold || state.label !== 'Unlabeled')

  function toggleGold() {
    if (!frame || !canGold || setGoldFrame.isPending) return
    const k = idKey(frame.id)
    const on = !isGold
    setGoldFrame.mutate(
      { frameId: frame.id, gold: on },
      { onSuccess: () => setGoldEdits((prev) => ({ ...prev, [k]: on })) },
    )
  }

  const reviewOf = (f: Frame) => {
    const k = idKey(f.id)
    return k in reviewEdits ? reviewEdits[k] : f.review
  }
  const openReview = frame ? reviewOf(frame) : undefined

  function setReview(f: Frame, r: Review | undefined) {
    setReviewEdits((prev) => ({ ...prev, [idKey(f.id)]: r ?? null }))
    void invalidateService(ReviewService)
  }

  function toggleFlag() {
    if (!frame || flagFrame.isPending) return
    const own = openReview?.reason === ReviewReason.FLAGGED && openReview.createdBy?.id === me?.id
    if (openReview && !own) {
      flash.error('Already in review', reasons[openReview.reason].label)
      return
    }
    const f = frame
    flagFrame.mutate(
      { frameId: f.id, flagged: !openReview },
      { onSuccess: (res) => setReview(f, res.review) },
    )
  }

  function resolve(resolution: ReviewResolution) {
    if (!frame || !openReview || resolveReview.isPending) return
    const f = frame
    resolveReview.mutate(
      { id: openReview.id, resolution },
      { onSuccess: () => setReview(f, undefined) },
    )
  }

  function nextDrift() {
    for (let d = 1; d <= frames.length; d++) {
      const i = (index + d) % frames.length
      if (review.drift.has(idKey(frames[i].id))) {
        seek(i)
        return
      }
    }
    flash.info('No drifted boxes in this clip')
  }

  function selectTrial(selected: bigint | undefined) {
    setParams(
      (prev) => {
        const next = new URLSearchParams(prev)
        if (selected === undefined) next.delete('trial')
        else next.set('trial', String(selected))
        return next
      },
      { replace: true },
    )
  }

  function nextModelDisagreement() {
    if (!trialReview) return
    const i = nextDisagreement(trialReview.verdicts, index)
    if (i >= 0) seek(i)
    else flash.info('No disagreements')
  }

  function cycleSelection(delta: number) {
    if (frameBoxes.length === 0) return
    const ids = frameBoxes.map((b) => b.trackId).sort((a, b) => (a < b ? -1 : 1))
    const at = ids.findIndex((t) => t === selectedTrackId)
    labels().select(ids[(at + delta + ids.length) % ids.length])
  }

  function deleteSelected() {
    if (!frame) return
    if (selectedTrackId === null) {
      if (frameBoxes.length > 0) flash.info('Select a box first')
      return
    }
    void labels().deleteBox(selectedTrackId, frame.id)
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
          v: toggleCorrected,
          '[': () => goClip(data?.previousClipId),
          ']': () => goClip(data?.nextClipId),
          n: nextFromQueue,
          '?': () => setShowHelp(true),
          ...typeKeys,
          Escape: () => labels().select(null),
          Tab: () => cycleSelection(1),
          'shift+Tab': () => cycleSelection(-1),
          Delete: deleteSelected,
          Backspace: deleteSelected,
          'shift+Delete': () =>
            selectedTrackId !== null && void labels().deleteTrack(selectedTrackId),
          'shift+Backspace': () =>
            selectedTrackId !== null && void labels().deleteTrack(selectedTrackId),
          c: async () => {
            if (!frame || index === 0) return
            const n = await labels().copyBoxes(frames[index - 1].id, frame.id)
            if (n === 0)
              flash.info('Nothing to copy', 'Every box on the previous frame is already here.')
          },
          r: () => {
            if (!frame || selectedTrackId === null) return
            const box = boxes[idKey(frame.id)]?.[idKey(selectedTrackId)]?.box
            if (box) void labels().trackObject(frame.id, { box }, selectedTrackId)
          },
          X: () => {
            if (frame && selectedTrackId !== null)
              void labels().searchExemplar(frame.id, selectedTrackId)
          },
          h: () => labels().toggleHidden(),
          d: nextDrift,
          b: nextModelDisagreement,
          B: () => trial && useTrialStore.getState().toggleVisible(),
          g: toggleGold,
          m: toggleFlag,
          ' ': (e) => mark(FrameStatus.LABELED, true, e),
          e: (e) => mark(FrameStatus.EMPTY, true, e),
          u: (e) => mark(FrameStatus.UNLABELED, false, e),
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
  const otherLabeler =
    takenByOther(clip.claim, me?.id) && takenClipId !== clip.id ? clip.claim?.user?.name : undefined

  const navbar = (
    <Navbar>
      <BackItem to="/" label="Home" />
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
          <Link
            to={`/videos/${video.id}`}
            className="text-zinc-500 hover:text-zinc-950 max-md:hidden dark:text-zinc-400 dark:hover:text-white"
          >
            {video.name} ·{' '}
          </Link>
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
        {otherLabeler && <Badge color="amber">{otherLabeler} is labeling this</Badge>}
        <SaveIndicator />
        {openReview && (
          <Badge color={reasons[openReview.reason].color}>
            <FlagIcon className="size-3" />
            {reasons[openReview.reason].label}
          </Badge>
        )}
        {openReview && canReview && (
          <Dropdown>
            <DropdownButton outline disabled={switching || resolveReview.isPending}>
              Resolve
            </DropdownButton>
            <DropdownMenu anchor="bottom end">
              {resolutions.map((r) => (
                <DropdownItem key={r.resolution} onClick={() => resolve(r.resolution)}>
                  {r.label}
                </DropdownItem>
              ))}
            </DropdownMenu>
          </Dropdown>
        )}
        <Badge color={state.color}>{state.label}</Badge>
        <Button
          plain
          title="Mark empty (E)"
          disabled={switching || frameBoxes.length > 0 || status === FrameStatus.EMPTY}
          onClick={() => mark(FrameStatus.EMPTY, true)}
        >
          <NoSymbolIcon data-slot="icon" />
          Empty
        </Button>
        <Button
          color="emerald"
          disabled={switching}
          title="Mark labeled and continue (Space)"
          onClick={() => mark(FrameStatus.LABELED, true)}
        >
          <CheckIcon data-slot="icon" />
          Done
        </Button>
        {finished && (
          <Button
            color="sky"
            disabled={switching || claimNext.isPending}
            title="Next clip (N)"
            onClick={nextFromQueue}
          >
            <ForwardIcon data-slot="icon" />
            Next clip
          </Button>
        )}
      </NavbarSection>
      <NavbarDivider className="max-lg:hidden" />
      <NavbarSection className="max-lg:hidden">
        <span className="text-sm/6 text-zinc-500 tabular-nums dark:text-zinc-400">
          Frame <span className="text-zinc-950 dark:text-white">{index + 1}</span> / {frames.length}
        </span>
        <NavbarItem
          aria-label={openReview ? 'Unflag frame' : 'Flag frame for review'}
          title="Flag for review (M)"
          onClick={toggleFlag}
        >
          <FlagIcon data-slot="icon" className={openReview ? 'fill-amber-400!' : undefined} />
        </NavbarItem>
        {canReview && (
          <NavbarItem
            aria-label={isGold ? 'Remove gold frame' : 'Make gold frame'}
            title="Gold frame (G)"
            disabled={!canGold}
            onClick={toggleGold}
          >
            <StarIcon data-slot="icon" className={isGold ? 'fill-amber-400!' : undefined} />
          </NavbarItem>
        )}
        <NavbarItem
          aria-label={hideBoxes ? 'Show boxes' : 'Hide boxes'}
          title="Hide boxes (H)"
          onClick={() => labels().toggleHidden()}
        >
          {hideBoxes ? <EyeSlashIcon data-slot="icon" /> : <EyeIcon data-slot="icon" />}
        </NavbarItem>
        <NavbarItem
          aria-label="Model trials"
          aria-pressed={showTrials}
          title="Model trials"
          onClick={() => setShowTrials(!showTrials)}
        >
          <BeakerIcon data-slot="icon" className={showTrials ? 'fill-sky-400!' : undefined} />
        </NavbarItem>
        <NavbarItem
          aria-label="Underwater correction"
          aria-pressed={corrected}
          title="Underwater correction (V)"
          onClick={toggleCorrected}
        >
          <SparklesIcon data-slot="icon" className={corrected ? 'fill-sky-400!' : undefined} />
        </NavbarItem>
        <NavbarItem aria-label="Fit to screen" title="Fit to screen (F)" onClick={fit}>
          <span className="min-w-10 text-center tabular-nums">{zoomPct}%</span>
        </NavbarItem>
        <NavbarItem aria-label="Keyboard shortcuts" onClick={() => setShowHelp(true)}>
          <QuestionMarkCircleIcon data-slot="icon" />
        </NavbarItem>
        {canDelete && (
          <NavbarItem
            aria-label="Move clip to trash"
            title="Move to trash"
            onClick={() => setDeleting(true)}
          >
            <TrashIcon data-slot="icon" />
          </NavbarItem>
        )}
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
          {canDelete && (
            <SidebarItem onClick={() => setDeleting(true)}>
              <TrashIcon data-slot="icon" />
              <SidebarLabel>Move to trash</SidebarLabel>
            </SidebarItem>
          )}
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
            <LabelCanvas
              tracks={tracks}
              types={types}
              drift={review.drift}
              overlay={<TrialLayer types={types} />}
            />
            {switching && (
              <div className="absolute inset-0 flex items-center justify-center bg-zinc-950/40">
                <LoadingSpinner />
              </div>
            )}
          </main>
          <TrackPanel
            tracks={tracks}
            types={types}
            unconfirmed={review.unconfirmed}
            top={
              showTrials && (
                <TrialPanel
                  clipId={clip.id}
                  types={types}
                  review={trialReview}
                  selectedId={trialId}
                  onSelect={selectTrial}
                />
              )
            }
          />
        </div>

        <Timeline
          frameClasses={frameBars}
          trackClasses={selectedBars}
          flagged={driftFlags}
          marked={reviewMarks}
          trialClasses={trialVisible ? trialBars : undefined}
        />
        {/* Reviewer edits never take the claim, so they only get the badge. */}
        <ConfirmAlert
          open={!!otherLabeler && !canReview && !switching}
          onClose={() => navigate(`/videos/${video.id}`)}
          onConfirm={() => takeClip.mutate({ clipId: clip.id })}
          busy={takeClip.isPending}
          error={takeClip.error}
          title="Label this clip anyway?"
          description={`${otherLabeler} is already working on this clip.`}
          confirmLabel="Label anyway"
        />
        <ConfirmAlert
          open={deleting}
          onClose={() => {
            setDeleting(false)
            deleteClip.reset()
          }}
          onConfirm={() => deleteClip.mutate({ id: clip.id })}
          busy={deleteClip.isPending}
          error={deleteClip.error}
          title={`Move clip ${clip.index + 1} to trash?`}
          description={plural(boxCount, 'box', 'boxes')}
          confirmLabel="Move to trash"
        />
        <ShortcutsDialog
          open={showHelp}
          onClose={() => setShowHelp(false)}
          groups={[
            labelingShortcuts,
            canReview
              ? { ...frameShortcuts, items: [...frameShortcuts.items, goldShortcut] }
              : frameShortcuts,
            navigationShortcuts,
            viewShortcuts,
            trialShortcuts,
          ]}
        />
      </div>
    </ClipShell>
  )
}
