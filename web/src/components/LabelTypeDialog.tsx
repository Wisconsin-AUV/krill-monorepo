import { useMutation } from '@connectrpc/connect-query'
import * as Headless from '@headlessui/react'
import { PlusIcon, XMarkIcon } from '@heroicons/react/20/solid'
import { clsx } from 'clsx'
import { useState } from 'react'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import {
  Dialog,
  DialogActions,
  DialogBody,
  DialogDescription,
  DialogTitle,
} from '@/components/ui/Dialog'
import { Description, Field, FieldGroup, Fieldset, Label, Legend } from '@/components/ui/Fieldset'
import { Input } from '@/components/ui/Input'
import { Text } from '@/components/ui/Text'
import { Textarea } from '@/components/ui/Textarea'
import { LabelService, type LabelType } from '@/gen/krill/v1/label_pb'
import { errorMessage } from '@/lib/errors'
import { invalidateService } from '@/lib/queryClient'
import { exportClasses, palette } from '@/lib/labels'

interface AttributeDraft {
  id: number
  name: string
  options: string
}

let nextId = 0

function toDrafts(type?: LabelType): AttributeDraft[] {
  return (type?.attributes ?? []).map((a) => ({
    id: nextId++,
    name: a.name,
    options: a.options.join(', '),
  }))
}

function parseOptions(raw: string): string[] {
  return raw
    .split(',')
    .map((o) => o.trim().toLowerCase())
    .filter(Boolean)
}

function LabelTypeForm({ type, onClose }: { type?: LabelType; onClose: () => void }) {
  const [name, setName] = useState(type?.name ?? '')
  const [color, setColor] = useState(type?.color ?? palette[0])
  const [description, setDescription] = useState(type?.description ?? '')
  const [attributes, setAttributes] = useState<AttributeDraft[]>(() => toDrafts(type))
  const [error, setError] = useState<string | null>(null)

  const onSuccess = async () => {
    await invalidateService(LabelService)
    onClose()
  }
  const onError = (err: unknown) => setError(errorMessage(err))
  const create = useMutation(LabelService.method.createLabelType, { onSuccess, onError })
  const update = useMutation(LabelService.method.updateLabelType, { onSuccess, onError })
  const busy = create.isPending || update.isPending

  const parsed = attributes
    .filter((a) => a.name.trim())
    .map((a) => ({ name: a.name.trim().toLowerCase(), options: parseOptions(a.options) }))
  const classes = exportClasses(name.trim().toLowerCase() || 'name', parsed)

  function submit(e: React.FormEvent) {
    e.preventDefault()
    setError(null)
    const body = { name, color, description, attributes: parsed }
    if (type) update.mutate({ id: type.id, ...body })
    else create.mutate(body)
  }

  const patch = (id: number, p: Partial<AttributeDraft>) =>
    setAttributes((all) => all.map((a) => (a.id === id ? { ...a, ...p } : a)))

  return (
    <form onSubmit={submit}>
      <DialogTitle>{type ? `Edit ${type.name}` : 'New label type'}</DialogTitle>
      <DialogDescription>
        Attributes are set once per track and inherited by every frame in it.
      </DialogDescription>
      <DialogBody>
        <FieldGroup>
          <Field>
            <Label>Name</Label>
            <Description>
              Lowercase snake_case, such as torpedo_hole. Used in exported class names.
            </Description>
            <Input
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="torpedo_hole"
              required
              maxLength={40}
              autoComplete="off"
            />
          </Field>

          <Fieldset>
            <Legend>Color</Legend>
            <Headless.RadioGroup
              value={color}
              onChange={setColor}
              aria-label="Color"
              className="mt-3 flex flex-wrap gap-2"
            >
              {palette.map((c) => (
                <Headless.Radio
                  key={c}
                  value={c}
                  aria-label={c}
                  className={clsx(
                    'size-7 cursor-pointer rounded-full ring-offset-2 ring-offset-white transition focus:outline-none data-focus:outline-2 data-focus:outline-offset-2 data-focus:outline-sky-500 dark:ring-offset-zinc-900',
                    'not-data-checked:hover:scale-110 data-checked:ring-2 data-checked:ring-zinc-950 dark:data-checked:ring-white',
                  )}
                  style={{ backgroundColor: c }}
                />
              ))}
            </Headless.RadioGroup>
          </Fieldset>

          <Field>
            <Label>Description</Label>
            <Description>
              What counts as this object. Link to the labeling guideline for edge cases.
            </Description>
            <Textarea
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              rows={2}
            />
          </Field>

          <Fieldset>
            <Legend>Attributes</Legend>
            <Text>For example size with options big, small. Options are comma separated.</Text>
            <div className="mt-4 space-y-3">
              {attributes.map((a) => (
                <div key={a.id} className="flex items-start gap-2">
                  <Input
                    aria-label="Attribute name"
                    placeholder="size"
                    value={a.name}
                    onChange={(e) => patch(a.id, { name: e.target.value })}
                    className="max-w-40"
                  />
                  <Input
                    aria-label="Options"
                    placeholder="big, small"
                    value={a.options}
                    onChange={(e) => patch(a.id, { options: e.target.value })}
                  />
                  <Button
                    plain
                    aria-label="Remove attribute"
                    onClick={() => setAttributes((all) => all.filter((x) => x.id !== a.id))}
                  >
                    <XMarkIcon data-slot="icon" />
                  </Button>
                </div>
              ))}
              <Button
                outline
                onClick={() =>
                  setAttributes((all) => [...all, { id: nextId++, name: '', options: '' }])
                }
              >
                <PlusIcon data-slot="icon" />
                Add attribute
              </Button>
            </div>
          </Fieldset>

          <div>
            <p className="text-sm/6 font-medium text-zinc-950 dark:text-white">
              Exports as {classes.length} {classes.length === 1 ? 'class' : 'classes'}
            </p>
            <div className="mt-2 flex flex-wrap gap-1.5">
              {classes.slice(0, 12).map((c) => (
                <Badge key={c} className="font-mono">
                  {c}
                </Badge>
              ))}
              {classes.length > 12 && <Badge>+{classes.length - 12} more</Badge>}
            </div>
          </div>

          {error && (
            <p className="rounded-lg bg-red-50 px-3 py-2 text-sm/6 text-red-700 dark:bg-red-500/10 dark:text-red-400">
              {error}
            </p>
          )}
        </FieldGroup>
      </DialogBody>
      <DialogActions>
        <Button plain onClick={onClose}>
          Cancel
        </Button>
        <Button type="submit" color="sky" disabled={busy || !name.trim()}>
          {type ? 'Save' : 'Create'}
        </Button>
      </DialogActions>
    </form>
  )
}

export function LabelTypeDialog({
  open,
  type,
  onClose,
}: {
  open: boolean
  type?: LabelType
  onClose: () => void
}) {
  return (
    <Dialog open={open} onClose={onClose} size="xl">
      {open && <LabelTypeForm type={type} onClose={onClose} />}
    </Dialog>
  )
}
