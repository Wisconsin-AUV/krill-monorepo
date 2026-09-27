import { create } from '@bufbuild/protobuf'
import { useMutation, useQuery } from '@connectrpc/connect-query'
import { CheckIcon, ChevronLeftIcon, ForwardIcon } from '@heroicons/react/20/solid'
import { useEffect, useMemo } from 'react'
import { useParams } from 'react-router'
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
import { Sidebar } from '@/components/ui/Sidebar'
import { StackedLayout } from '@/components/ui/StackedLayout'
import { FrameStatus } from '@/gen/krill/v1/annotation_pb'
import { FrameSchema, GetClipResponseSchema } from '@/gen/krill/v1/clip_pb'
import { GoldService } from '@/gen/krill/v1/gold_pb'
import { LabelService } from '@/gen/krill/v1/label_pb'
import { errorMessage } from '@/lib/errors'
import { flash } from '@/lib/flash'
import { useClaimNextClip } from '@/lib/queue'
import type { TrackInfo } from '@/labeling/AnnotationLayer'
import { resultBoxes } from '@/labeling/goldResult'
import { GoldResultLayer } from '@/labeling/GoldResultLayer'
import { LabelCanvas } from '@/labeling/LabelCanvas'
import { localBackend } from '@/labeling/localBackend'
import { TrackPanel } from '@/labeling/TrackPanel'
import { TypePanel } from '@/labeling/TypePanel'
import { idKey, useLabelStore } from '@/labeling/useLabelStore'
import { FrameCanvas } from '@/workspace/FrameCanvas'
import { useHotkeys } from '@/workspace/useHotkeys'
import { useWorkspaceStore } from '@/workspace/useWorkspaceStore'

const noDrift = new Map<string, Set<string>>()
const noneUnconfirmed = new Set<string>()

function Shell({ actions, children }: { actions?: React.ReactNode; children: React.ReactNode }) {
  return (
    <StackedLayout
      fill
      navbar={
        <Navbar>
          <NavbarItem to="/" current={false} className="max-lg:hidden">
            <ChevronLeftIcon data-slot="icon" />
            <NavbarLabel>Home</NavbarLabel>
          </NavbarItem>
          <NavbarDivider className="max-lg:hidden" />
          <span className="text-sm/6 font-medium">Gold check</span>
          <NavbarSpacer />
          <NavbarSection>{actions}</NavbarSection>
        </Navbar>
      }
      sidebar={<Sidebar />}
    >
      {children}
    </StackedLayout>
  )
}

export function GoldCheckPage() {
  const raw = useParams().id
  const frameId = raw && /^\d+$/.test(raw) ? BigInt(raw) : undefined
  const claimNext = useClaimNextClip()

  const { data, error, isPending } = useQuery(
    GoldService.method.getGoldCheck,
    { frameId },
    { enabled: frameId !== undefined, staleTime: Infinity, gcTime: 0, retry: false },
  )
  const { data: typeData } = useQuery(LabelService.method.listLabelTypes, {})
  const submit = useMutation(GoldService.method.submitGoldCheck, {
    onError: (err) => flash.error('Could not submit', err),
  })
  const result = submit.data?.result

  const types = useMemo(() => typeData?.labelTypes ?? [], [typeData])
  const view = useWorkspaceStore((s) => s.view)
  const imageSize = useWorkspaceStore((s) => s.imageSize)
  const fit = useWorkspaceStore((s) => s.fit)
  const trackMap = useLabelStore((s) => s.tracks)
  const boxes = useLabelStore((s) => s.boxes)
  const selectedTrackId = useLabelStore((s) => s.selectedTrackId)
  const labels = useLabelStore.getState

  useEffect(() => {
    if (!data) return
    const frame = create(FrameSchema, {
      id: data.frameId,
      url: data.url,
      status: FrameStatus.UNLABELED,
    })
    useWorkspaceStore.getState().setFrames([frame], 0)
    useWorkspaceStore.getState().setImageSize({ width: data.width, height: data.height })
    labels().load(
      create(GetClipResponseSchema, { clip: { id: data.frameId }, frames: [frame] }),
      localBackend(),
    )
  }, [data, labels])

  useEffect(() => {
    const active = labels().activeTypeId
    if (types.length > 0 && (active === null || !types.some((t) => t.id === active))) {
      labels().setActiveType(types[0].id)
    }
  }, [types, labels])

  useEffect(() => {
    document.title = 'Gold check · Krill'
  }, [])

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

  const frameBoxes = data ? Object.values(boxes[idKey(data.frameId)] ?? {}) : []

  function send() {
    if (!data || result || submit.isPending || labels().pending > 0) return
    submit.mutate({
      frameId: data.frameId,
      boxes: frameBoxes.map((a) => {
        const t = trackMap[idKey(a.trackId)]
        return { labelTypeId: t?.labelTypeId, attributes: t?.attributes, box: a.box }
      }),
    })
  }

  const next = () => !claimNext.isPending && claimNext.mutate({})

  function cycleSelection(delta: number) {
    if (frameBoxes.length === 0) return
    const ids = frameBoxes.map((b) => b.trackId).sort((a, b) => (a < b ? -1 : 1))
    const at = ids.findIndex((t) => t === selectedTrackId)
    labels().select(ids[(at + delta + ids.length) % ids.length])
  }

  const typeKeys = Object.fromEntries(
    types.slice(0, 9).map((t, i) => [String(i + 1), () => labels().setActiveType(t.id)]),
  )
  const deleteSelected = () =>
    data && selectedTrackId !== null && void labels().deleteBox(selectedTrackId, data.frameId)

  useHotkeys(
    !data
      ? {}
      : result
        ? { f: fit, n: next }
        : {
            f: fit,
            h: () => labels().toggleHidden(),
            ...typeKeys,
            Escape: () => labels().select(null),
            Tab: () => cycleSelection(1),
            'shift+Tab': () => cycleSelection(-1),
            Delete: deleteSelected,
            Backspace: deleteSelected,
            'mod+z': () => void labels().undo(),
            ' ': send,
          },
  )

  if (frameId === undefined || error) {
    return (
      <Shell>
        <div className="flex flex-1 flex-col items-center justify-center gap-4 py-24 text-zinc-700 dark:text-zinc-300">
          <p>{error ? errorMessage(error) : 'Gold frame not found.'}</p>
          <Button color="sky" disabled={claimNext.isPending} onClick={next}>
            Next clip
          </Button>
        </div>
      </Shell>
    )
  }
  if (isPending) {
    return (
      <Shell>
        <div className="flex flex-1 items-center justify-center py-24">
          <LoadingSpinner />
        </div>
      </Shell>
    )
  }

  const outcomes = result ? resultBoxes(result, types) : []
  const count = (o: string) => outcomes.filter((r) => r.outcome === o).length

  const actions = result ? (
    <>
      <span className="text-sm text-zinc-500 tabular-nums max-md:hidden dark:text-zinc-400">
        {count('correct')} correct · {count('off')} off · {count('missed')} missed ·{' '}
        {count('extra')} extra
      </span>
      <Badge color={result.score >= 0.8 ? 'green' : result.score >= 0.5 ? 'amber' : 'red'}>
        {Math.round(result.score * 100)}%
      </Badge>
      <Button color="sky" disabled={claimNext.isPending} title="Next clip (N)" onClick={next}>
        <ForwardIcon data-slot="icon" />
        Next clip
      </Button>
    </>
  ) : (
    <Button
      color="emerald"
      disabled={submit.isPending}
      title={frameBoxes.length === 0 ? 'Submit as empty (Space)' : 'Submit (Space)'}
      onClick={send}
    >
      <CheckIcon data-slot="icon" />
      {frameBoxes.length === 0 ? 'Submit empty' : 'Submit'}
    </Button>
  )

  return (
    <Shell actions={actions}>
      <div className="flex min-h-0 flex-1 text-zinc-950 dark:text-zinc-100">
        {!result && <TypePanel types={types} counts={typeCounts} />}
        <main className="relative min-w-0 flex-1 bg-zinc-950 bg-[radial-gradient(var(--color-zinc-800)_1px,transparent_1px)] [background-size:16px_16px]">
          {result ? (
            <FrameCanvas>
              <GoldResultLayer
                result={result}
                types={types}
                imageSize={imageSize}
                scale={view.scale}
              />
            </FrameCanvas>
          ) : (
            <LabelCanvas tracks={tracks} types={types} drift={noDrift} canTrack={false} />
          )}
        </main>
        {!result && (
          <TrackPanel
            tracks={tracks}
            types={types}
            unconfirmed={noneUnconfirmed}
            canTrack={false}
          />
        )}
      </div>
    </Shell>
  )
}
