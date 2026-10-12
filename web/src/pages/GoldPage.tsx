import { timestampDate } from '@bufbuild/protobuf/wkt'
import { useInfiniteQuery, useMutation, useQuery } from '@connectrpc/connect-query'
import { StarIcon } from '@heroicons/react/24/outline'
import { TrashIcon } from '@heroicons/react/16/solid'
import { useState } from 'react'
import { Link } from 'react-router'
import { EmptyState } from '@/components/EmptyState'
import { Thumbnail } from '@/components/Thumbnail'
import { Avatar } from '@/components/ui/Avatar'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { QueryError } from '@/components/ui/Callout'
import { Dialog, DialogBody, DialogTitle } from '@/components/ui/Dialog'
import { Heading, Subheading } from '@/components/ui/Heading'
import LoadingSpinner from '@/components/ui/LoadingSpinner'
import PageContentBlock from '@/components/ui/PageContentBlock'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/Table'
import { Text } from '@/components/ui/Text'
import { GoldService } from '@/gen/krill/v1/gold_pb'
import { initials } from '@/lib/auth'
import { flash } from '@/lib/flash'
import { formatRelative, plural } from '@/lib/format'
import { invalidateService } from '@/lib/queryClient'
import { scoreColor } from '@/labeling/goldResult'

const pct = (n: number) => `${Math.round(n * 100)}%`

export function GoldPage() {
  const { data, isPending, error } = useQuery(GoldService.method.getGoldStats, {})
  const [shown, setShown] = useState<AttemptFilter | null>(null)
  const remove = useMutation(GoldService.method.setGoldFrame, {
    onSuccess: () => invalidateService(GoldService),
    onError: (err) => flash.error('Could not remove gold frame', err),
  })

  return (
    <PageContentBlock title="Gold · Krill">
      <Heading>Gold</Heading>

      {isPending ? (
        <div className="flex justify-center py-16">
          <LoadingSpinner />
        </div>
      ) : error ? (
        <QueryError title="Could not load gold frames" error={error} className="mt-8" />
      ) : data.frames.length === 0 ? (
        <div className="mt-8">
          <EmptyState
            icon={StarIcon}
            title="No gold frames"
            description="Press G on a labeled frame to make it gold."
          />
        </div>
      ) : (
        <>
          <Subheading className="mt-10">Labelers</Subheading>
          {data.labelers.length === 0 ? (
            <Text className="mt-2">No checks answered yet.</Text>
          ) : (
            <Table className="mt-4 [--gutter:--spacing(6)] lg:[--gutter:--spacing(10)]">
              <TableHead>
                <TableRow>
                  <TableHeader>Labeler</TableHeader>
                  <TableHeader className="text-right">Checks</TableHeader>
                  <TableHeader className="text-right">Accuracy</TableHeader>
                  <TableHeader className="text-right">Mean IoU</TableHeader>
                </TableRow>
              </TableHead>
              <TableBody>
                {data.labelers.map((l) => (
                  <TableRow
                    key={l.user?.id}
                    className="cursor-pointer hover:bg-zinc-950/[2.5%] dark:hover:bg-white/[2.5%]"
                    onClick={() => setShown({ title: l.user?.name ?? '', userId: l.user?.id })}
                  >
                    <TableCell>
                      <div className="flex items-center gap-3">
                        <Avatar
                          initials={initials(l.user?.name ?? '')}
                          className="size-8 rounded-full bg-sky-600"
                        />
                        <button
                          type="button"
                          className="font-medium text-zinc-950 focus:outline-none focus-visible:underline dark:text-white"
                        >
                          {l.user?.name}
                        </button>
                        {l.needsReview && <Badge color="amber">Needs review</Badge>}
                      </div>
                    </TableCell>
                    <TableCell className="text-right tabular-nums">{l.attempts}</TableCell>
                    <TableCell className="text-right tabular-nums">{pct(l.accuracy)}</TableCell>
                    <TableCell className="text-right tabular-nums">
                      {l.meanIou > 0 ? l.meanIou.toFixed(2) : '–'}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}

          <Subheading className="mt-10">Frames</Subheading>
          <ul className="mt-4 grid gap-6 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
            {data.frames.map((f) => (
              <li key={String(f.frameId)}>
                <button
                  type="button"
                  className="block w-full"
                  onClick={() =>
                    setShown({
                      title: `${f.videoName} · Frame ${f.index + 1}`,
                      frameId: f.frameId,
                    })
                  }
                >
                  <Thumbnail src={f.thumbnailUrl} alt="" />
                </button>
                <div className="mt-2 flex items-start justify-between gap-2">
                  <div className="min-w-0 text-sm/5">
                    <Link
                      to={`/clips/${f.clipId}?frame=${f.index}`}
                      className="block truncate font-medium text-zinc-950 hover:underline dark:text-white"
                    >
                      {f.videoName} · Frame {f.index + 1}
                    </Link>
                    <div className="text-zinc-500 tabular-nums dark:text-zinc-400">
                      {plural(f.boxCount, 'box', 'boxes')} · {plural(f.attempts, 'check')}
                      {f.attempts > 0 && ` · ${pct(f.meanScore)}`}
                    </div>
                  </div>
                  <Button
                    plain
                    title="Remove gold frame"
                    disabled={remove.isPending}
                    onClick={() => remove.mutate({ frameId: f.frameId, gold: false })}
                  >
                    <TrashIcon data-slot="icon" />
                  </Button>
                </div>
              </li>
            ))}
          </ul>
        </>
      )}

      <Dialog open={shown !== null} onClose={() => setShown(null)}>
        {shown && <AttemptList filter={shown} />}
      </Dialog>
    </PageContentBlock>
  )
}

interface AttemptFilter {
  title: string
  userId?: string
  frameId?: bigint
}

function AttemptList({ filter }: { filter: AttemptFilter }) {
  const { data, isPending, error, fetchNextPage, hasNextPage, isFetchingNextPage } =
    useInfiniteQuery(
      GoldService.method.listGoldAttempts,
      { userId: filter.userId ?? '', frameId: filter.frameId ?? 0n, pageToken: '' },
      { pageParamKey: 'pageToken', getNextPageParam: (p) => p.nextPageToken || undefined },
    )
  const attempts = data?.pages.flatMap((p) => p.attempts) ?? []

  return (
    <>
      <DialogTitle>{filter.title}</DialogTitle>
      <DialogBody>
        {isPending ? (
          <div className="flex justify-center py-8">
            <LoadingSpinner />
          </div>
        ) : error ? (
          <QueryError title="Could not load checks" error={error} />
        ) : attempts.length === 0 ? (
          <Text>No checks answered yet.</Text>
        ) : (
          <ul className="divide-y divide-zinc-950/5 dark:divide-white/5">
            {attempts.map((a) => (
              <li key={`${a.frameId}-${a.user?.id}`}>
                <Link
                  to={`/gold/attempts/${a.frameId}/${a.user?.id}`}
                  className="-mx-2 flex items-center justify-between gap-4 rounded-md px-2 py-3 hover:bg-zinc-950/[2.5%] dark:hover:bg-white/[2.5%]"
                >
                  <div className="min-w-0 text-sm/5">
                    <div className="truncate font-medium text-zinc-950 dark:text-white">
                      {filter.userId ? `${a.videoName} · Frame ${a.index + 1}` : a.user?.name}
                    </div>
                    <div className="text-zinc-500 tabular-nums dark:text-zinc-400">
                      {a.createdAt && formatRelative(timestampDate(a.createdAt))}
                      {a.meanIou > 0 && ` · IoU ${a.meanIou.toFixed(2)}`}
                    </div>
                  </div>
                  <Badge color={scoreColor(a.score)}>{pct(a.score)}</Badge>
                </Link>
              </li>
            ))}
          </ul>
        )}
        {hasNextPage && (
          <div className="mt-4 flex justify-center">
            <Button outline disabled={isFetchingNextPage} onClick={() => void fetchNextPage()}>
              Load more
            </Button>
          </div>
        )}
      </DialogBody>
    </>
  )
}
