import { ChevronRightIcon } from '@heroicons/react/20/solid'
import { clsx } from 'clsx'
import { useState } from 'react'
import { Field, Fieldset, Label, Legend } from '@/components/ui/Fieldset'
import { Input } from '@/components/ui/Input'
import { augmentationFields, type AugmentationKey, type AugmentationValues } from '@/admin/pipeline'

export function NumberField({
  label,
  value,
  onChange,
  min,
  max,
  step,
}: {
  label: string
  value: number
  onChange: (v: number) => void
  min: number
  max: number
  step?: number | 'any'
}) {
  return (
    <Field>
      <Label>{label}</Label>
      <Input
        type="number"
        min={min}
        max={max}
        step={step}
        value={Number.isNaN(value) ? '' : value}
        onChange={(e) => onChange(e.target.valueAsNumber)}
        required
      />
    </Field>
  )
}

export function AugmentationFields({
  value,
  onChange,
  omit = [],
  className = 'sm:grid-cols-3',
}: {
  value: AugmentationValues
  onChange: (v: AugmentationValues) => void
  omit?: AugmentationKey[]
  className?: string
}) {
  const [open, setOpen] = useState(false)
  return (
    <Fieldset>
      <Legend>
        <button
          type="button"
          aria-expanded={open}
          onClick={() => setOpen((o) => !o)}
          className="flex items-center gap-2 rounded-sm focus:outline-none focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-500"
        >
          <ChevronRightIcon
            className={clsx(
              'size-4 fill-zinc-400 transition-transform duration-150 dark:fill-zinc-500',
              open && 'rotate-90',
            )}
          />
          Augmentation
        </button>
      </Legend>
      {open && (
        <div className={clsx('mt-6 grid gap-6', className)}>
          {augmentationFields
            .filter((f) => !omit.includes(f.key))
            .map((f) => (
              <NumberField
                key={f.key}
                label={f.label}
                value={value[f.key]}
                onChange={(v) => onChange({ ...value, [f.key]: v })}
                min={f.min}
                max={f.max}
                step={f.step}
              />
            ))}
        </div>
      )}
    </Fieldset>
  )
}
