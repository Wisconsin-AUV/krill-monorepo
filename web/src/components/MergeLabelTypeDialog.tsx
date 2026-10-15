import { useMutation, useQuery } from '@connectrpc/connect-query'
import { ArrowRightIcon } from '@heroicons/react/16/solid'
import { useState } from 'react'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { FormError } from '@/components/ui/Callout'
import { Dialog, DialogActions, DialogBody, DialogTitle } from '@/components/ui/Dialog'
import { Field, Label } from '@/components/ui/Fieldset'
import { Subheading } from '@/components/ui/Heading'
import LoadingSpinner from '@/components/ui/LoadingSpinner'
import { Select } from '@/components/ui/Select'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/Table'
import { GoldService } from '@/gen/krill/v1/gold_pb'
import { LabelService, type LabelType } from '@/gen/krill/v1/label_pb'
import { flash } from '@/lib/flash'
import { formatNumber, plural } from '@/lib/format'
import { invalidateService } from '@/lib/queryClient'

interface Mapping {
  sourceAttribute: string
  sourceOption: string
  targetAttribute: string
  targetOption: string
  trackCount: number
}

// Select values pair an attribute with an option. Names never contain a NUL.
const join = (attribute: string, option: string) => (attribute ? `${attribute}\0${option}` : '')

export function MergeLabelTypeDialog({
  source,
  types,
  onClose,
}: {
  source: LabelType | null
  types: LabelType[]
  onClose: () => void
}) {
  const [targetId, setTargetId] = useState('')
  // Null until the first preview fills in the default mapping.
  const [mappings, setMappings] = useState<Mapping[] | null>(null)
  const target = types.find((t) => String(t.id) === targetId)
  const open = source !== null

  const preview = useQuery(
    LabelService.method.previewMergeLabelTypes,
    { sourceId: source?.id, targetId: target?.id, mappings: mappings ?? [] },
    { enabled: open && target !== undefined, placeholderData: (prev) => prev },
  )
  const data = preview.data
  const shown: Mapping[] = mappings ?? data?.mappings ?? []

  const merge = useMutation(LabelService.method.mergeLabelTypes, {
    onSuccess: async (res) => {
      await Promise.all([invalidateService(LabelService), invalidateService(GoldService)])
      flash.success(`Merged ${plural(res.trackCount, 'track')} into ${target?.name ?? ''}`)
      close()
    },
  })
  const close = () => {
    merge.reset()
    setTargetId('')
    setMappings(null)
    onClose()
  }

  const setTarget = (i: number, value: string) => {
    const [attribute = '', option = ''] = value.split('\0')
    setMappings(
      shown.map((m, j) =>
        j === i ? { ...m, targetAttribute: attribute, targetOption: option } : m,
      ),
    )
  }
  const dropped = shown.filter((m) => !m.targetAttribute && m.trackCount > 0)
  const renamed = data?.classChanges.filter((c) => c.newId >= 0) ?? []
  const merged = data?.classChanges.filter((c) => c.newId < 0) ?? []

  return (
    <Dialog open={open} onClose={close} size="2xl">
      <DialogTitle>Merge {source?.name}</DialogTitle>
      <DialogBody>
        <Field>
          <Label>Into</Label>
          <Select
            value={targetId}
            onChange={(e) => {
              setTargetId(e.target.value)
              setMappings(null)
            }}
          >
            <option value="" disabled>
              Choose a label type
            </option>
            {types
              .filter((t) => t.id !== source?.id)
              .map((t) => (
                <option key={String(t.id)} value={String(t.id)}>
                  {t.name}
                </option>
              ))}
          </Select>
        </Field>

        {target && !data && preview.isFetching && (
          <div className="flex justify-center py-8">
            <LoadingSpinner />
          </div>
        )}
        {target && data && (
          <>
            <dl className="mt-6 grid grid-cols-2 gap-x-6 gap-y-1 text-sm/6 sm:grid-cols-4">
              {(
                [
                  ['Tracks', data.trackCount],
                  ['Boxes', data.boxCount],
                  ['Clips', data.clipCount],
                  ['Videos', data.videoCount],
                  ['In trash', data.trashedTrackCount],
                  ['Gold frames', data.goldFrameCount],
                  ['Gold attempts', data.goldAttemptCount],
                  ['Open reviews', data.openReviewCount],
                ] as const
              ).map(([label, n]) => (
                <div key={label}>
                  <dt className="text-zinc-500 dark:text-zinc-400">{label}</dt>
                  <dd className="font-medium text-zinc-950 tabular-nums dark:text-white">
                    {formatNumber(n)}
                  </dd>
                </div>
              ))}
            </dl>

            {shown.length > 0 && (
              <>
                <Subheading className="mt-8">Attributes</Subheading>
                <Table dense className="mt-2 [--gutter:--spacing(1)]">
                  <TableHead>
                    <TableRow>
                      <TableHeader>{source?.name}</TableHeader>
                      <TableHeader className="text-right">Tracks</TableHeader>
                      <TableHeader>
                        <span className="sr-only">To</span>
                      </TableHeader>
                      <TableHeader>{target.name}</TableHeader>
                    </TableRow>
                  </TableHead>
                  <TableBody>
                    {shown.map((m, i) => (
                      <TableRow key={join(m.sourceAttribute, m.sourceOption)}>
                        <TableCell className="font-mono">
                          {m.sourceAttribute}={m.sourceOption}
                        </TableCell>
                        <TableCell className="text-right tabular-nums">
                          {formatNumber(m.trackCount)}
                        </TableCell>
                        <TableCell>
                          <ArrowRightIcon className="size-4 text-zinc-400" />
                        </TableCell>
                        <TableCell>
                          <Select
                            aria-label={`${m.sourceAttribute}=${m.sourceOption} becomes`}
                            value={join(m.targetAttribute, m.targetOption)}
                            onChange={(e) => setTarget(i, e.target.value)}
                          >
                            <option value="">Drop</option>
                            {target.attributes.map((a) => (
                              <optgroup key={a.name} label={a.name}>
                                {a.options.map((o) => (
                                  <option key={o} value={join(a.name, o)}>
                                    {a.name}={o}
                                  </option>
                                ))}
                              </optgroup>
                            ))}
                          </Select>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </>
            )}

            {(dropped.length > 0 || data.unfilledAttributes.length > 0) && (
              <div className="mt-4 flex flex-wrap gap-2">
                {dropped.map((m) => (
                  <Badge key={join(m.sourceAttribute, m.sourceOption)} color="amber">
                    Drops {m.sourceAttribute}={m.sourceOption} on {plural(m.trackCount, 'track')}
                  </Badge>
                ))}
                {data.unfilledAttributes.map((a) => (
                  <Badge key={a} color="amber">
                    No {a}
                  </Badge>
                ))}
              </div>
            )}

            <Subheading className="mt-8">Export classes</Subheading>
            <ul className="mt-2 space-y-1 font-mono text-sm/6 text-zinc-700 dark:text-zinc-300">
              {merged.map((c) => (
                <li key={c.name}>
                  {c.name} <span className="text-zinc-400">#{c.oldId}</span> →{' '}
                  {c.becomes || <span className="text-amber-600">incomplete</span>}
                </li>
              ))}
              {renamed.length > 0 && (
                <li className="font-sans text-zinc-500 dark:text-zinc-400">
                  {plural(renamed.length, 'class', 'classes')} renumbered
                </li>
              )}
            </ul>
            {data.exampleCount > 0 && (
              <p className="mt-4 text-sm/6 text-zinc-500 dark:text-zinc-400">
                {plural(data.exampleCount, 'example image')} deleted
              </p>
            )}
          </>
        )}
        <FormError error={preview.error ?? merge.error} className="mt-6" />
      </DialogBody>
      <DialogActions>
        <Button plain onClick={close}>
          Cancel
        </Button>
        <Button
          color="red"
          disabled={!target || !data || preview.isError || preview.isFetching || merge.isPending}
          onClick={() =>
            source &&
            target &&
            merge.mutate({ sourceId: source.id, targetId: target.id, mappings: shown })
          }
        >
          Merge into {target?.name ?? '…'}
        </Button>
      </DialogActions>
    </Dialog>
  )
}
