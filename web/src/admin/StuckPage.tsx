import { timestampDate } from '@bufbuild/protobuf/wkt'
import { useMutation, useQuery } from '@connectrpc/connect-query'
import { useState } from 'react'
import { Link } from 'react-router'
import { ConfirmAlert } from '@/components/ConfirmAlert'
import { Meter } from '@/components/Meter'
import { Avatar } from '@/components/ui/Avatar'
import { Button } from '@/components/ui/Button'
import { QueryError } from '@/components/ui/Callout'
import { Checkbox } from '@/components/ui/Checkbox'
import { Heading } from '@/components/ui/Heading'
import LoadingSpinner from '@/components/ui/LoadingSpinner'
import PageContentBlock from '@/components/ui/PageContentBlock'
import { Select } from '@/components/ui/Select'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/Table'
import { InsightService } from '@/gen/krill/v1/insight_pb'
import { flash } from '@/lib/flash'
import { formatNumber, formatRelative, plural } from '@/lib/format'
import { invalidateService } from '@/lib/queryClient'

const dayOptions = [1, 2, 3, 5, 7, 14, 30]

export function StuckPage() {
  // Zero asks for the KRILL_STUCK_CLAIM_DAYS default.
  const [days, setDays] = useState(0)
  const [selected, setSelected] = useState<Set<bigint>>(new Set())
  const [confirming, setConfirming] = useState(false)
  const { data, isPending, error } = useQuery(InsightService.method.listStuckClips, {
    idleDays: days,
  })
  const clips = data?.clips ?? []
  const idleDays = data?.idleDays ?? days
  const chosen = clips.filter((c) => selected.has(c.clipId))
  const options = [...new Set([...dayOptions, idleDays])].filter((d) => d > 0).sort((a, b) => a - b)

  const release = useMutation(InsightService.method.releaseStuckClips, {
    onSuccess: async (res) => {
      setConfirming(false)
      setSelected(new Set())
      flash.success(`Released ${plural(res.released, 'clip')}`)
      await invalidateService(InsightService)
    },
  })

  const toggle = (id: bigint, on: boolean) =>
    setSelected((s) => {
      const next = new Set(s)
      if (on) next.add(id)
      else next.delete(id)
      return next
    })

  return (
    <PageContentBlock title="Stuck clips · Admin · Krill">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <Heading>Stuck clips</Heading>
        <div className="flex gap-3">
          <Select
            aria-label="Idle for"
            value={idleDays}
            onChange={(e) => {
              setDays(Number(e.target.value))
              setSelected(new Set())
            }}
            className="sm:w-44"
          >
            {options.map((d) => (
              <option key={d} value={d}>
                Idle {plural(d, 'day')}
              </option>
            ))}
          </Select>
          <Button
            color="red"
            disabled={chosen.length === 0}
            onClick={() => {
              release.reset()
              setConfirming(true)
            }}
          >
            Release{chosen.length > 0 && ` ${chosen.length}`}
          </Button>
        </div>
      </div>

      {isPending ? (
        <div className="flex justify-center py-16">
          <LoadingSpinner />
        </div>
      ) : error ? (
        <QueryError title="Could not load stuck clips" error={error} className="mt-8" />
      ) : clips.length === 0 ? (
        <p className="py-10 text-center text-sm/6 text-zinc-500 dark:text-zinc-400">
          No stuck clips
        </p>
      ) : (
        <Table className="mt-8 [--gutter:--spacing(6)] lg:[--gutter:--spacing(10)]">
          <TableHead>
            <TableRow>
              <TableHeader className="w-10">
                <Checkbox
                  aria-label="Select all"
                  checked={chosen.length === clips.length}
                  indeterminate={chosen.length > 0 && chosen.length < clips.length}
                  onChange={(on) => setSelected(new Set(on ? clips.map((c) => c.clipId) : []))}
                />
              </TableHeader>
              <TableHeader>Clip</TableHeader>
              <TableHeader>Claimed by</TableHeader>
              <TableHeader>Last edit</TableHeader>
              <TableHeader className="w-48">Progress</TableHeader>
              <TableHeader className="text-right">Boxes</TableHeader>
            </TableRow>
          </TableHead>
          <TableBody>
            {clips.map((c) => {
              const at = c.activeAt ? timestampDate(c.activeAt) : null
              return (
                <TableRow key={String(c.clipId)}>
                  <TableCell>
                    <Checkbox
                      aria-label="Select"
                      checked={selected.has(c.clipId)}
                      onChange={(on) => toggle(c.clipId, on)}
                    />
                  </TableCell>
                  <TableCell>
                    <Link
                      to={`/clips/${c.clipId}`}
                      className="font-medium text-zinc-950 hover:underline dark:text-white"
                    >
                      {c.videoName} · Clip {c.clipIndex + 1}
                    </Link>
                  </TableCell>
                  <TableCell>
                    <span className="flex items-center gap-2">
                      <Avatar seed={c.claimant?.id ?? ''} className="size-6" />
                      {c.claimant?.name}
                    </span>
                  </TableCell>
                  <TableCell
                    className="text-zinc-500 dark:text-zinc-400"
                    title={at?.toLocaleString()}
                  >
                    {at ? formatRelative(at) : ''}
                  </TableCell>
                  <TableCell>
                    <div className="flex items-center gap-3">
                      <Meter
                        label="Frames done"
                        value={c.frameCount > 0 ? c.labeledFrames / c.frameCount : 0}
                      />
                      <span className="text-xs/5 text-zinc-500 tabular-nums dark:text-zinc-400">
                        {c.labeledFrames}/{c.frameCount}
                      </span>
                    </div>
                  </TableCell>
                  <TableCell className="text-right tabular-nums">{formatNumber(c.boxes)}</TableCell>
                </TableRow>
              )
            })}
          </TableBody>
        </Table>
      )}

      <ConfirmAlert
        open={confirming}
        onClose={() => setConfirming(false)}
        onConfirm={() =>
          release.mutate({ clipIds: chosen.map((c) => c.clipId), idleDays: idleDays })
        }
        title={`Release ${plural(chosen.length, 'clip')}?`}
        description="Their labels stay."
        confirmLabel="Release"
        busy={release.isPending}
        error={release.error}
      />
    </PageContentBlock>
  )
}
