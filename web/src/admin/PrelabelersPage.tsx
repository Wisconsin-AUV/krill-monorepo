import { create, equals } from '@bufbuild/protobuf'
import { useMutation, useQuery } from '@connectrpc/connect-query'
import { AdjustmentsHorizontalIcon, PlayIcon } from '@heroicons/react/20/solid'
import { useState } from 'react'
import { EmptyState } from '@/components/EmptyState'
import { Meter } from '@/components/Meter'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { FormError, QueryError } from '@/components/ui/Callout'
import { Field, Label } from '@/components/ui/Fieldset'
import { Heading, Subheading } from '@/components/ui/Heading'
import { Input } from '@/components/ui/Input'
import LoadingSpinner from '@/components/ui/LoadingSpinner'
import PageContentBlock from '@/components/ui/PageContentBlock'
import { Select } from '@/components/ui/Select'
import { Switch } from '@/components/ui/Switch'
import { LabelService, type LabelType } from '@/gen/krill/v1/label_pb'
import {
  ParamKind,
  ParamValueSchema,
  PipelineService,
  PrelabelerOutputSchema,
  PrelabelSource,
  type ParamSpec,
  type ParamValue,
  type Prelabeler,
  type PrelabelerOutput,
} from '@/gen/krill/v1/pipeline_pb'
import { flash } from '@/lib/flash'
import { formatNumber, plural } from '@/lib/format'
import { invalidateService } from '@/lib/queryClient'
import { HsvRangeInput } from './HsvRangeInput'
import { PrelabelPreview } from './PrelabelPreview'
import { prelabelActive, prelabelStatus } from './pipeline'

const POLL = 5000

function ParamInput({
  spec,
  value,
  onChange,
}: {
  spec: ParamSpec
  value: ParamValue
  onChange: (v: ParamValue) => void
}) {
  const v = value.value
  switch (spec.kind) {
    case ParamKind.HSV_RANGE:
      return v.case === 'hsv' ? (
        <div>
          <p className="text-sm/6 font-medium text-zinc-950 dark:text-white">{spec.label}</p>
          <div className="mt-2">
            <HsvRangeInput
              value={v.value}
              onChange={(hsv) =>
                onChange(
                  create(ParamValueSchema, { name: spec.name, value: { case: 'hsv', value: hsv } }),
                )
              }
            />
          </div>
        </div>
      ) : null
    case ParamKind.BOOL:
      return (
        <div className="flex items-center justify-between gap-4">
          <span className="text-sm/6 text-zinc-950 dark:text-white">{spec.label}</span>
          <Switch
            aria-label={spec.label}
            checked={v.case === 'flag' && v.value}
            onChange={(on) =>
              onChange(
                create(ParamValueSchema, { name: spec.name, value: { case: 'flag', value: on } }),
              )
            }
          />
        </div>
      )
    default:
      return (
        <Field>
          <Label>{spec.label}</Label>
          <Input
            type="number"
            min={spec.max > spec.min ? spec.min : undefined}
            max={spec.max > spec.min ? spec.max : undefined}
            step={spec.step || (spec.kind === ParamKind.INT ? 1 : 'any')}
            value={v.case === 'number' ? v.value : ''}
            onChange={(e) =>
              onChange(
                create(ParamValueSchema, {
                  name: spec.name,
                  value: { case: 'number', value: e.target.valueAsNumber || 0 },
                }),
              )
            }
          />
        </Field>
      )
  }
}

function OutputRow({
  output,
  types,
  onChange,
}: {
  output: PrelabelerOutput
  types: LabelType[]
  onChange: (o: PrelabelerOutput) => void
}) {
  const type = types.find((t) => t.id === output.labelTypeId)
  const set = (patch: Partial<PrelabelerOutput>) =>
    onChange(create(PrelabelerOutputSchema, { ...output, ...patch }))
  return (
    <div className="grid items-end gap-3 sm:grid-cols-[6rem_1fr_1fr]">
      <span className="pb-2 text-sm/6 font-medium text-zinc-950 dark:text-white">
        {output.name}
      </span>
      <Field>
        <Label className="sr-only">Label for {output.name}</Label>
        <Select
          value={String(output.labelTypeId)}
          onChange={(e) => set({ labelTypeId: BigInt(e.target.value), attributes: {} })}
        >
          <option value="0">Off</option>
          {types.map((t) => (
            <option key={String(t.id)} value={String(t.id)}>
              {t.title || t.name}
            </option>
          ))}
        </Select>
      </Field>
      <div className="flex flex-wrap gap-3">
        {type?.attributes.map((a) => (
          <Select
            key={a.name}
            aria-label={a.name}
            value={output.attributes[a.name] ?? ''}
            onChange={(e) => {
              const attributes = { ...output.attributes }
              if (e.target.value) attributes[a.name] = e.target.value
              else delete attributes[a.name]
              set({ attributes })
            }}
          >
            <option value="">{a.name}: unset</option>
            {a.options.map((o) => (
              <option key={o} value={o}>
                {a.name}: {o}
              </option>
            ))}
          </Select>
        ))}
      </div>
    </div>
  )
}

function same<T extends ParamValue | PrelabelerOutput>(
  schema: typeof ParamValueSchema | typeof PrelabelerOutputSchema,
  a: T[],
  b: T[],
) {
  return (
    a.length === b.length && a.every((x, i) => equals(schema as never, x as never, b[i] as never))
  )
}

function PrelabelerCard({ saved, types }: { saved: Prelabeler; types: LabelType[] }) {
  const spec = saved.spec!
  const [enabled, setEnabled] = useState(saved.enabled)
  const [params, setParams] = useState(saved.params)
  const [outputs, setOutputs] = useState(saved.outputs)
  const dirty =
    enabled !== saved.enabled ||
    !same(ParamValueSchema, params, saved.params) ||
    !same(PrelabelerOutputSchema, outputs, saved.outputs)

  const save = useMutation(PipelineService.method.updatePrelabeler, {
    // The saved prelabeler remounts this card with its new state.
    onSuccess: async () => {
      flash.success(`${spec.title} saved`)
      await invalidateService(PipelineService)
    },
  })
  const colors = Object.fromEntries(
    outputs.map((o) => [o.name, types.find((t) => t.id === o.labelTypeId)?.color ?? '#facc15']),
  )

  return (
    <section className="rounded-xl ring-1 ring-zinc-950/10 dark:ring-white/10">
      <header className="flex flex-wrap items-center gap-4 border-b border-zinc-950/5 p-6 dark:border-white/5">
        <Subheading className="flex-1">{spec.title}</Subheading>
        <span className="flex items-center gap-3 text-sm/6 text-zinc-500 dark:text-zinc-400">
          Enabled
          <Switch aria-label="Enabled" checked={enabled} onChange={setEnabled} />
        </span>
        {dirty && (
          <Button
            plain
            onClick={() => {
              setEnabled(saved.enabled)
              setParams(saved.params)
              setOutputs(saved.outputs)
            }}
          >
            Discard
          </Button>
        )}
        <Button
          color="sky"
          disabled={!dirty || save.isPending}
          onClick={() => save.mutate({ name: spec.name, enabled, params, outputs })}
        >
          Save
        </Button>
      </header>
      <div className="grid gap-10 p-6 xl:grid-cols-[minmax(0,5fr)_minmax(0,7fr)]">
        <div className="space-y-8">
          <FormError error={save.error} />
          <div className="space-y-3">
            <h3 className="text-sm/6 font-semibold text-zinc-950 dark:text-white">Labels</h3>
            {outputs.map((o, i) => (
              <OutputRow
                key={o.name}
                output={o}
                types={types}
                onChange={(next) => setOutputs(outputs.map((x, j) => (j === i ? next : x)))}
              />
            ))}
          </div>
          <div className="space-y-6">
            <h3 className="text-sm/6 font-semibold text-zinc-950 dark:text-white">Thresholds</h3>
            {spec.params.map((ps, i) => (
              <ParamInput
                key={ps.name}
                spec={ps}
                value={params[i]}
                onChange={(v) => setParams(params.map((x, j) => (j === i ? v : x)))}
              />
            ))}
          </div>
        </div>
        <div>
          <h3 className="mb-3 text-sm/6 font-semibold text-zinc-950 dark:text-white">
            Test on a frame
          </h3>
          <PrelabelPreview name={spec.name} params={params} colors={colors} />
        </div>
      </div>
    </section>
  )
}

export function PrelabelersPage() {
  const list = useQuery(PipelineService.method.listPrelabelers, {})
  const types = useQuery(LabelService.method.listLabelTypes, {})
  const runs = useQuery(
    PipelineService.method.listPrelabelRuns,
    {},
    {
      refetchInterval: (q) =>
        q.state.data?.runs.some((r) => prelabelActive(r.status)) ? POLL : false,
    },
  )
  const start = useMutation(PipelineService.method.startPrelabel, {
    onSuccess: () => invalidateService(PipelineService),
  })
  const cancel = useMutation(PipelineService.method.cancelPrelabel, {
    onSuccess: () => invalidateService(PipelineService),
  })
  const latest = runs.data?.runs.find((r) => r.source === PrelabelSource.CV)
  const active = latest !== undefined && prelabelActive(latest.status)
  const anyEnabled = list.data?.prelabelers.some((p) => p.enabled) ?? false

  return (
    <PageContentBlock title="Pre-labelers · Admin · Krill">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <Heading>Pre-labelers</Heading>
        <div className="flex items-center gap-4">
          {latest && (
            <span className="flex items-center gap-2 text-sm/6 text-zinc-500 tabular-nums dark:text-zinc-400">
              <Badge color={prelabelStatus[latest.status].color}>
                {prelabelStatus[latest.status].label}
              </Badge>
              {latest.clipsDone}/{latest.clipsTotal} clips ·{' '}
              {plural(latest.proposals, 'box', 'boxes')}
              {runs.data && ` · ${formatNumber(runs.data.openCvProposals)} to check`}
            </span>
          )}
          {active ? (
            <Button
              outline
              onClick={() => cancel.mutate({ id: latest.id })}
              disabled={cancel.isPending}
            >
              Cancel
            </Button>
          ) : (
            <Button
              color="sky"
              onClick={() => start.mutate({ source: PrelabelSource.CV })}
              disabled={!anyEnabled || start.isPending}
            >
              <PlayIcon data-slot="icon" />
              Pre-label
            </Button>
          )}
        </div>
      </div>
      {active && latest.clipsTotal > 0 && (
        <Meter label="Pre-labeling" value={latest.clipsDone / latest.clipsTotal} className="mt-4" />
      )}
      <FormError error={start.error ?? cancel.error ?? latest?.error} className="mt-4" />
      {list.isPending || types.isPending ? (
        <div className="flex justify-center py-16">
          <LoadingSpinner />
        </div>
      ) : list.error ? (
        <QueryError title="Could not load pre-labelers" error={list.error} className="mt-8" />
      ) : list.data.prelabelers.length === 0 ? (
        <div className="mt-8">
          <EmptyState
            icon={AdjustmentsHorizontalIcon}
            title="No pre-labelers"
            description="They appear once a GPU worker connects."
          />
        </div>
      ) : (
        <div className="mt-8 space-y-8">
          {list.data.prelabelers.map((p) => (
            <PrelabelerCard
              key={`${p.spec?.name}-${p.updatedAt?.seconds ?? 0}`}
              saved={p}
              types={types.data?.labelTypes ?? []}
            />
          ))}
        </div>
      )}
    </PageContentBlock>
  )
}
