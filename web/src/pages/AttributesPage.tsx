import { useInfiniteQuery, useMutation, useQuery } from '@connectrpc/connect-query'
import { Squares2X2Icon } from '@heroicons/react/24/outline'
import { clsx } from 'clsx'
import { useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router'
import { EmptyState } from '@/components/EmptyState'
import { Button } from '@/components/ui/Button'
import { Checkbox, CheckboxField } from '@/components/ui/Checkbox'
import { Label } from '@/components/ui/Fieldset'
import { Heading } from '@/components/ui/Heading'
import LoadingSpinner from '@/components/ui/LoadingSpinner'
import PageContentBlock from '@/components/ui/PageContentBlock'
import { SegmentedTabs } from '@/components/SegmentedTabs'
import { Select } from '@/components/ui/Select'
import { Text } from '@/components/ui/Text'
import { AnnotationService, type TrackCrop } from '@/gen/krill/v1/annotation_pb'
import { LabelService, type LabelType } from '@/gen/krill/v1/label_pb'
import { VideoService } from '@/gen/krill/v1/video_pb'
import { errorMessage } from '@/lib/errors'
import { flash } from '@/lib/flash'
import { missingAttributes } from '@/lib/labels'
import { loadFrame } from '@/workspace/imageCache'
import { Kbd } from '@/workspace/ShortcutsDialog'
import { useHotkeys } from '@/workspace/useHotkeys'

const CROP_SIZE = 176
// Context around the box so the labeler can tell similar objects apart.
const CROP_PAD = 0.25
const PREFETCH_WITHIN = 12

function CropCanvas({ crop, color }: { crop: TrackCrop; color: string }) {
  const ref = useRef<HTMLCanvasElement>(null)

  useEffect(() => {
    let cancelled = false
    loadFrame(crop.frameId, crop.frameUrl)
      .ready.then((img) => {
        const canvas = ref.current
        const box = crop.box
        if (cancelled || !canvas || !box) return
        const dpr = window.devicePixelRatio || 1
        canvas.width = CROP_SIZE * dpr
        canvas.height = CROP_SIZE * dpr
        const ctx = canvas.getContext('2d')
        if (!ctx) return

        const W = img.naturalWidth
        const H = img.naturalHeight
        const bw = box.width * W
        const bh = box.height * H
        const side = Math.max(bw, bh) * (1 + 2 * CROP_PAD)
        const sx = box.x * W + bw / 2 - side / 2
        const sy = box.y * H + bh / 2 - side / 2
        const scale = canvas.width / side

        ctx.fillStyle = '#18181b'
        ctx.fillRect(0, 0, canvas.width, canvas.height)
        ctx.drawImage(img, sx, sy, side, side, 0, 0, canvas.width, canvas.height)
        ctx.strokeStyle = color
        ctx.lineWidth = 1.5 * dpr
        ctx.strokeRect((box.x * W - sx) * scale, (box.y * H - sy) * scale, bw * scale, bh * scale)
      })
      .catch(() => undefined)
    return () => {
      cancelled = true
    }
  }, [crop, color])

  return (
    <canvas
      ref={ref}
      className="block aspect-square w-full rounded-md bg-zinc-900"
      width={CROP_SIZE}
      height={CROP_SIZE}
    />
  )
}

function CropGrid({
  type,
  videoId,
  missingOnly,
}: {
  type: LabelType
  videoId: bigint
  missingOnly: boolean
}) {
  const navigate = useNavigate()
  const [selected, setSelected] = useState(0)
  const [attrIndex, setAttrIndex] = useState(0)
  // Saved values stay local so a finished track keeps its place in the grid
  // instead of vanishing from the missing-only list mid-pass.
  const [edits, setEdits] = useState<Record<string, Record<string, string>>>({})
  const gridRef = useRef<HTMLUListElement>(null)
  const itemRefs = useRef<(HTMLLIElement | null)[]>([])
  const saving = useRef(new Map<string, Promise<unknown>>())

  const { data, isPending, error, fetchNextPage, hasNextPage, isFetchingNextPage } =
    useInfiniteQuery(
      AnnotationService.method.listTracksByType,
      { labelTypeId: type.id, videoId, missingAttributes: missingOnly, pageToken: '' },
      { pageParamKey: 'pageToken', getNextPageParam: (p) => p.nextPageToken || undefined },
    )
  const crops = useMemo(() => data?.pages.flatMap((p) => p.tracks) ?? [], [data])
  const update = useMutation(AnnotationService.method.updateTrack)

  const attr = type.attributes[attrIndex]
  const valuesOf = (c: TrackCrop) => edits[String(c.track?.id)] ?? c.track?.attributes ?? {}

  useEffect(() => {
    itemRefs.current[selected]?.scrollIntoView({ block: 'nearest' })
    if (hasNextPage && !isFetchingNextPage && selected >= crops.length - PREFETCH_WITHIN) {
      void fetchNextPage()
    }
  }, [selected, crops.length, hasNextPage, isFetchingNextPage, fetchNextPage])

  const move = (delta: number) => {
    if (crops.length === 0) return
    setSelected((i) => Math.min(crops.length - 1, Math.max(0, i + delta)))
  }

  const columns = () => {
    const grid = gridRef.current
    if (!grid) return 1
    return getComputedStyle(grid).gridTemplateColumns.split(' ').length
  }

  const save = (crop: TrackCrop, value: string | undefined) => {
    const track = crop.track
    if (!track || !attr) return
    const key = String(track.id)
    const prev = valuesOf(crop)
    const next = { ...prev }
    if (value) next[attr.name] = value
    else delete next[attr.name]
    setEdits((e) => ({ ...e, [key]: next }))
    // Each save replaces every attribute, so saves to one track must land in order.
    const run = (saving.current.get(key) ?? Promise.resolve()).then(() =>
      update.mutateAsync({ id: track.id, attributes: next, setAttributes: true }),
    )
    saving.current.set(
      key,
      run.catch(() => undefined),
    )
    run.catch((err: unknown) => {
      setEdits((e) => (e[key] === next ? { ...e, [key]: prev } : e))
      flash.error('Could not save attribute', err)
    })
  }

  const setOption = (n: number) => {
    const crop = crops[selected]
    const option = attr?.options[n]
    if (!crop || !option) return
    save(crop, option)
    move(1)
  }

  const cycleAttribute = (delta: number) => {
    const n = type.attributes.length
    if (n > 0) setAttrIndex((i) => (i + delta + n) % n)
  }

  const digitKeys = Object.fromEntries(
    Array.from({ length: 9 }, (_, i) => [String(i + 1), () => setOption(i)]),
  )
  useHotkeys({
    ...digitKeys,
    ArrowLeft: () => move(-1),
    ArrowRight: () => move(1),
    j: () => move(-1),
    k: () => move(1),
    ArrowUp: () => move(-columns()),
    ArrowDown: () => move(columns()),
    Home: () => move(-crops.length),
    End: () => move(crops.length),
    Backspace: () => crops[selected] && save(crops[selected], undefined),
    Tab: () => cycleAttribute(1),
    'shift+Tab': () => cycleAttribute(-1),
    Enter: () => {
      const crop = crops[selected]
      if (crop?.track) navigate(`/clips/${crop.track.clipId}?frame=${crop.frameIndex}`)
    },
  })

  if (isPending) {
    return (
      <div className="flex justify-center py-16">
        <LoadingSpinner />
      </div>
    )
  }
  if (error) {
    return (
      <Text className="mt-8 text-red-600 dark:text-red-400">
        Could not load tracks: {errorMessage(error)}
      </Text>
    )
  }
  if (crops.length === 0) {
    return (
      <div className="mt-8">
        <EmptyState
          icon={Squares2X2Icon}
          title={missingOnly ? 'Nothing missing' : 'No tracks'}
          description={
            missingOnly ? `Every ${type.name} track has its attributes.` : `No ${type.name} tracks.`
          }
        />
      </div>
    )
  }

  return (
    <>
      <div className="mt-6 flex flex-wrap items-center gap-x-6 gap-y-3">
        {type.attributes.length > 1 && (
          <div className="flex items-center gap-2">
            <SegmentedTabs
              options={type.attributes.map((a, i) => ({ key: String(i), label: a.name }))}
              value={String(attrIndex)}
              onChange={(k) => setAttrIndex(Number(k))}
            />
            <Kbd>Tab</Kbd>
          </div>
        )}
        {attr && (
          <div className="flex flex-wrap items-center gap-x-4 gap-y-2 text-sm/6 text-zinc-700 dark:text-zinc-300">
            {attr.options.slice(0, 9).map((o, i) => (
              <span key={o} className="flex items-center gap-1.5">
                <Kbd>{i + 1}</Kbd>
                {o}
              </span>
            ))}
            <span className="flex items-center gap-1.5 text-zinc-500 dark:text-zinc-400">
              <Kbd>⌫</Kbd>
              clear
            </span>
          </div>
        )}
      </div>

      <ul ref={gridRef} className="mt-4 grid grid-cols-[repeat(auto-fill,minmax(9rem,1fr))] gap-3">
        {crops.map((c, i) => {
          const values = valuesOf(c)
          const missing = missingAttributes(type, values)
          return (
            <li
              key={String(c.track?.id)}
              ref={(el) => {
                itemRefs.current[i] = el
              }}
            >
              <button
                type="button"
                onClick={() => setSelected(i)}
                onDoubleClick={() =>
                  c.track && navigate(`/clips/${c.track.clipId}?frame=${c.frameIndex}`)
                }
                className={clsx(
                  'block w-full rounded-lg p-1 text-left ring-2 transition-shadow focus:outline-hidden',
                  i === selected ? 'ring-sky-500' : 'ring-transparent hover:ring-zinc-950/10',
                  missing.length === 0 && 'opacity-60',
                )}
              >
                <CropCanvas crop={c} color={type.color} />
                <div className="mt-1 space-y-0.5 px-0.5 text-xs/5">
                  {type.attributes.map((a) => (
                    <div
                      key={a.name}
                      className={clsx(
                        'flex justify-between gap-2',
                        a.name === attr?.name
                          ? 'font-medium text-zinc-950 dark:text-white'
                          : 'text-zinc-500 dark:text-zinc-400',
                      )}
                    >
                      <span className="truncate">{a.name}</span>
                      <span
                        className={clsx(
                          'truncate',
                          !values[a.name] && 'text-amber-600 dark:text-amber-400',
                        )}
                      >
                        {values[a.name] ?? '–'}
                      </span>
                    </div>
                  ))}
                </div>
              </button>
            </li>
          )
        })}
      </ul>

      {hasNextPage && (
        <div className="mt-6 flex justify-center">
          <Button outline disabled={isFetchingNextPage} onClick={() => void fetchNextPage()}>
            Load more
          </Button>
        </div>
      )}
    </>
  )
}

export function AttributesPage() {
  const [params, setParams] = useSearchParams()
  const { data: typeData, isPending, error } = useQuery(LabelService.method.listLabelTypes, {})
  const { data: videoData } = useQuery(VideoService.method.listVideos, {})

  const types = useMemo(
    () => (typeData?.labelTypes ?? []).filter((t) => t.attributes.length > 0),
    [typeData],
  )
  const type = types.find((t) => String(t.id) === params.get('type')) ?? types[0]
  const videoParam = params.get('video') ?? ''
  const videoId = /^\d+$/.test(videoParam) ? BigInt(videoParam) : 0n
  const missingOnly = params.get('all') !== '1'

  const setParam = (key: string, value: string | null) =>
    setParams(
      (p) => {
        if (value === null) p.delete(key)
        else p.set(key, value)
        return p
      },
      { replace: true },
    )

  return (
    <PageContentBlock title="Attributes · Krill" width="max-w-7xl p-6 lg:p-10">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <Heading>Attributes</Heading>
        {type && (
          <div className="flex flex-wrap items-center gap-4">
            <Select
              aria-label="Label type"
              className="w-44"
              value={String(type.id)}
              onChange={(e) => setParam('type', e.target.value)}
            >
              {types.map((t) => (
                <option key={String(t.id)} value={String(t.id)}>
                  {t.name}
                </option>
              ))}
            </Select>
            <Select
              aria-label="Video"
              className="w-56"
              value={String(videoId)}
              onChange={(e) => setParam('video', e.target.value === '0' ? null : e.target.value)}
            >
              <option value="0">All videos</option>
              {videoData?.videos.map((v) => (
                <option key={String(v.id)} value={String(v.id)}>
                  {v.name}
                </option>
              ))}
            </Select>
            <CheckboxField>
              <Checkbox checked={missingOnly} onChange={(on) => setParam('all', on ? null : '1')} />
              <Label>Missing only</Label>
            </CheckboxField>
          </div>
        )}
      </div>

      {isPending ? (
        <div className="flex justify-center py-16">
          <LoadingSpinner />
        </div>
      ) : error ? (
        <Text className="mt-8 text-red-600 dark:text-red-400">
          Could not load label types: {errorMessage(error)}
        </Text>
      ) : !type ? (
        <div className="mt-8">
          <EmptyState
            icon={Squares2X2Icon}
            title="No attributes"
            description="No label type has attributes."
          />
        </div>
      ) : (
        <CropGrid
          key={`${type.id}-${videoId}-${missingOnly}`}
          type={type}
          videoId={videoId}
          missingOnly={missingOnly}
        />
      )}
    </PageContentBlock>
  )
}
