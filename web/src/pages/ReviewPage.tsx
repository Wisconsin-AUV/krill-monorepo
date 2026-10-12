import { useQuery } from '@connectrpc/connect-query'
import { timestampDate } from '@bufbuild/protobuf/wkt'
import { FlagIcon } from '@heroicons/react/24/outline'
import { useSearchParams } from 'react-router'
import { EmptyState } from '@/components/EmptyState'
import { SegmentedTabs } from '@/components/SegmentedTabs'
import { Thumbnail } from '@/components/Thumbnail'
import { Badge } from '@/components/ui/Badge'
import { Heading } from '@/components/ui/Heading'
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
import { ReviewService } from '@/gen/krill/v1/review_pb'
import { errorMessage } from '@/lib/errors'
import { formatRelative } from '@/lib/format'
import { reasonFromKey, reasonTabs, reasons } from '@/lib/reviews'

export function ReviewPage() {
  const [params, setParams] = useSearchParams()
  const reason = reasonFromKey(params.get('reason'))
  const { data, isPending, error } = useQuery(ReviewService.method.listReviews, { reason })

  return (
    <PageContentBlock title="Review · Krill">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <Heading>Review</Heading>
        <SegmentedTabs
          options={reasonTabs}
          value={reasons[reason].key}
          onChange={(key) => setParams(key === 'all' ? {} : { reason: key }, { replace: true })}
        />
      </div>

      {isPending ? (
        <div className="flex justify-center py-16">
          <LoadingSpinner />
        </div>
      ) : error ? (
        <Text className="mt-8 text-red-600 dark:text-red-400">
          Could not load reviews: {errorMessage(error)}
        </Text>
      ) : data.reviews.length === 0 ? (
        <div className="mt-8">
          <EmptyState
            icon={FlagIcon}
            title="Nothing to review"
            description="Frames flagged for review show up here."
          />
        </div>
      ) : (
        <Table className="mt-8 [--gutter:--spacing(6)] lg:[--gutter:--spacing(10)]">
          <TableHead>
            <TableRow>
              <TableHeader>Frame</TableHeader>
              <TableHeader>Reason</TableHeader>
              <TableHeader>Labeler</TableHeader>
              <TableHeader className="text-right">Queued</TableHeader>
            </TableRow>
          </TableHead>
          <TableBody>
            {data.reviews.map((r) => {
              const why = reasons[r.review?.reason ?? 0]
              const by = r.review?.createdBy
              return (
                <TableRow
                  key={String(r.review?.id)}
                  href={`/clips/${r.clipId}?frame=${r.frameIndex}`}
                >
                  <TableCell>
                    <div className="flex items-center gap-4">
                      <Thumbnail src={r.thumbnailUrl} className="w-24 shrink-0" />
                      <div className="min-w-0 text-sm/5">
                        <div className="truncate font-medium text-zinc-950 dark:text-white">
                          {r.videoName}
                        </div>
                        <div className="text-zinc-500 tabular-nums dark:text-zinc-400">
                          Clip {r.clipIndex + 1} · Frame {r.frameIndex + 1}
                        </div>
                      </div>
                    </div>
                  </TableCell>
                  <TableCell>
                    <Badge color={why.color}>{why.label}</Badge>
                    {by && by.id !== r.labeler?.id && (
                      <span className="ml-2 text-zinc-500 dark:text-zinc-400">by {by.name}</span>
                    )}
                  </TableCell>
                  <TableCell>{r.labeler?.name ?? '–'}</TableCell>
                  <TableCell className="text-right text-zinc-500 dark:text-zinc-400">
                    {r.review?.createdAt && formatRelative(timestampDate(r.review.createdAt))}
                  </TableCell>
                </TableRow>
              )
            })}
          </TableBody>
        </Table>
      )}
    </PageContentBlock>
  )
}
