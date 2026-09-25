import { useMutation, useQuery } from '@connectrpc/connect-query'
import { StarIcon } from '@heroicons/react/24/outline'
import { TrashIcon } from '@heroicons/react/16/solid'
import { Link } from 'react-router'
import { EmptyState } from '@/components/EmptyState'
import { Thumbnail } from '@/components/Thumbnail'
import { Avatar } from '@/components/ui/Avatar'
import { Button } from '@/components/ui/Button'
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
import { errorMessage } from '@/lib/errors'
import { flash } from '@/lib/flash'
import { plural } from '@/lib/format'
import { invalidateService } from '@/lib/queryClient'
import { profilePath } from '@/lib/stats'

const pct = (n: number) => `${Math.round(n * 100)}%`

export function GoldPage() {
  const { data, isPending, error } = useQuery(GoldService.method.getGoldStats, {})
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
        <Text className="mt-8 text-red-600 dark:text-red-400">
          Could not load gold frames: {errorMessage(error)}
        </Text>
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
                  <TableRow key={l.user?.id} href={profilePath(l.user?.username ?? '')}>
                    <TableCell>
                      <div className="flex items-center gap-3">
                        <Avatar
                          initials={initials(l.user?.name ?? '')}
                          className="size-8 rounded-full bg-sky-600"
                        />
                        <span className="font-medium text-zinc-950 dark:text-white">
                          {l.user?.name}
                        </span>
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
                <Link to={`/clips/${f.clipId}?frame=${f.index}`}>
                  <Thumbnail src={f.thumbnailUrl} alt="" />
                </Link>
                <div className="mt-2 flex items-start justify-between gap-2">
                  <div className="min-w-0 text-sm/5">
                    <div className="truncate font-medium text-zinc-950 dark:text-white">
                      {f.videoName} · Frame {f.index + 1}
                    </div>
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
    </PageContentBlock>
  )
}
