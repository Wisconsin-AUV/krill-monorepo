import { useMutation, useQuery } from '@connectrpc/connect-query'
import {
  ArrowDownIcon,
  ArrowUpIcon,
  PencilSquareIcon,
  PlusIcon,
  SparklesIcon,
  TrashIcon,
} from '@heroicons/react/20/solid'
import { TagIcon } from '@heroicons/react/24/outline'
import { useState } from 'react'
import { ConfirmAlert } from '@/components/ConfirmAlert'
import { EmptyState } from '@/components/EmptyState'
import { LabelTypeDialog } from '@/components/LabelTypeDialog'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
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
import { LabelService, type LabelType } from '@/gen/krill/v1/label_pb'
import { labelClient } from '@/lib/clients'
import { errorMessage } from '@/lib/errors'
import { flash } from '@/lib/flash'
import { formatNumber } from '@/lib/format'
import { exportClasses, palette, starterTypes } from '@/lib/labels'
import { invalidateService } from '@/lib/queryClient'
import { Kbd } from '@/workspace/ShortcutsDialog'

export function LabelsPage() {
  const { data, isPending, error } = useQuery(LabelService.method.listLabelTypes, {})
  const [editing, setEditing] = useState<bigint | 'new' | null>(null)
  const [deleting, setDeleting] = useState<LabelType | null>(null)
  const [seeding, setSeeding] = useState(false)
  const types = data?.labelTypes ?? []

  const reorder = useMutation(LabelService.method.reorderLabelTypes, {
    onSuccess: () => invalidateService(LabelService),
    onError: (err) => flash.error('Could not reorder', err),
  })
  const remove = useMutation(LabelService.method.deleteLabelType, {
    onSuccess: async () => {
      await invalidateService(LabelService)
      setDeleting(null)
    },
    onError: (err) => {
      setDeleting(null)
      flash.error('Could not delete label type', err)
    },
  })

  function move(i: number, delta: number) {
    const ids = types.map((t) => t.id)
    const [id] = ids.splice(i, 1)
    ids.splice(i + delta, 0, id)
    reorder.mutate({ ids })
  }

  async function addStarterSet() {
    setSeeding(true)
    try {
      for (const [i, t] of starterTypes.entries()) {
        await labelClient.createLabelType({ ...t, color: palette[i % palette.length] })
      }
      flash.success('Starter label types added', 'Add roles once the season taxonomy is settled.')
    } catch (err) {
      flash.error('Could not add starter types', err)
    } finally {
      await invalidateService(LabelService)
      setSeeding(false)
    }
  }

  return (
    <PageContentBlock title="Labels · Krill">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <Heading>Labels</Heading>
          <Text className="mt-1">
            Object types and their track attributes. Press 1 to 9 while labeling to pick a type in
            this order.
          </Text>
        </div>
        {types.length > 0 && (
          <Button color="sky" onClick={() => setEditing('new')}>
            <PlusIcon data-slot="icon" />
            New label type
          </Button>
        )}
      </div>

      {isPending ? (
        <div className="flex justify-center py-16">
          <LoadingSpinner />
        </div>
      ) : error ? (
        <Text className="mt-8 text-red-600 dark:text-red-400">
          Could not load labels: {errorMessage(error)}
        </Text>
      ) : types.length === 0 ? (
        <div className="mt-10">
          <EmptyState
            icon={TagIcon}
            title="No label types yet"
            description="Start from the RoboSub task list in the design doc, or define your own."
          >
            <div className="flex flex-wrap justify-center gap-3">
              <Button outline onClick={addStarterSet} disabled={seeding}>
                <SparklesIcon data-slot="icon" />
                Add starter set
              </Button>
              <Button color="sky" onClick={() => setEditing('new')}>
                <PlusIcon data-slot="icon" />
                New label type
              </Button>
            </div>
          </EmptyState>
        </div>
      ) : (
        <Table className="mt-8 [--gutter:--spacing(6)] lg:[--gutter:--spacing(10)]">
          <TableHead>
            <TableRow>
              <TableHeader className="w-12">Key</TableHeader>
              <TableHeader>Type</TableHeader>
              <TableHeader>Attributes</TableHeader>
              <TableHeader className="text-right">Classes</TableHeader>
              <TableHeader className="text-right">Tracks</TableHeader>
              <TableHeader className="text-right">Boxes</TableHeader>
              <TableHeader className="relative w-0">
                <span className="sr-only">Actions</span>
              </TableHeader>
            </TableRow>
          </TableHead>
          <TableBody>
            {types.map((t, i) => (
              <TableRow key={String(t.id)}>
                <TableCell>
                  {i < 9 ? <Kbd>{i + 1}</Kbd> : <span className="text-zinc-400">—</span>}
                </TableCell>
                <TableCell>
                  <div className="flex items-center gap-3">
                    <span
                      className="size-3 shrink-0 rounded-full"
                      style={{ backgroundColor: t.color }}
                    />
                    <div className="min-w-0">
                      <div className="font-mono font-medium text-zinc-950 dark:text-white">
                        {t.name}
                      </div>
                      {t.description && (
                        <div className="max-w-xs truncate text-zinc-500 dark:text-zinc-400">
                          {t.description}
                        </div>
                      )}
                    </div>
                  </div>
                </TableCell>
                <TableCell>
                  <div className="flex flex-wrap gap-1.5">
                    {t.attributes.length === 0 && (
                      <span className="text-zinc-400 dark:text-zinc-500">None</span>
                    )}
                    {t.attributes.map((a) => (
                      <Badge key={a.name}>
                        <span className="font-medium">{a.name}</span>
                        <span className="text-zinc-500 dark:text-zinc-400">
                          {a.options.join(' · ')}
                        </span>
                      </Badge>
                    ))}
                  </div>
                </TableCell>
                <TableCell className="text-right tabular-nums">
                  {exportClasses(t.name, t.attributes).length}
                </TableCell>
                <TableCell className="text-right tabular-nums">
                  {formatNumber(t.trackCount)}
                </TableCell>
                <TableCell className="text-right tabular-nums">
                  {formatNumber(t.boxCount)}
                </TableCell>
                <TableCell>
                  <div className="flex justify-end gap-1">
                    <Button
                      plain
                      aria-label={`Move ${t.name} up`}
                      disabled={i === 0 || reorder.isPending}
                      onClick={() => move(i, -1)}
                    >
                      <ArrowUpIcon data-slot="icon" />
                    </Button>
                    <Button
                      plain
                      aria-label={`Move ${t.name} down`}
                      disabled={i === types.length - 1 || reorder.isPending}
                      onClick={() => move(i, 1)}
                    >
                      <ArrowDownIcon data-slot="icon" />
                    </Button>
                    <Button plain aria-label={`Edit ${t.name}`} onClick={() => setEditing(t.id)}>
                      <PencilSquareIcon data-slot="icon" />
                    </Button>
                    <Button
                      plain
                      aria-label={`Delete ${t.name}`}
                      title={t.trackCount > 0 ? 'In use by tracks' : undefined}
                      disabled={t.trackCount > 0}
                      onClick={() => setDeleting(t)}
                    >
                      <TrashIcon data-slot="icon" />
                    </Button>
                  </div>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}

      <LabelTypeDialog
        open={editing !== null}
        type={types.find((t) => t.id === editing)}
        onClose={() => setEditing(null)}
      />
      <ConfirmAlert
        open={deleting !== null}
        onClose={() => setDeleting(null)}
        onConfirm={() => deleting && remove.mutate({ id: deleting.id })}
        busy={remove.isPending}
        title={`Delete ${deleting?.name ?? ''}?`}
        description="No tracks use this type, so nothing else changes."
        confirmLabel="Delete"
      />
    </PageContentBlock>
  )
}
