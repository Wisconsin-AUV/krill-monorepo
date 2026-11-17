import { useMutation } from '@connectrpc/connect-query'
import * as Headless from '@headlessui/react'
import { ArrowPathIcon, PhotoIcon, PlusIcon, XMarkIcon } from '@heroicons/react/20/solid'
import { clsx } from 'clsx'
import { useRef, useState } from 'react'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { FormError } from '@/components/ui/Callout'
import { Dialog, DialogActions, DialogBody, DialogTitle } from '@/components/ui/Dialog'
import { Description, Field, FieldGroup, Fieldset, Label, Legend } from '@/components/ui/Fieldset'
import { Input } from '@/components/ui/Input'
import { Switch } from '@/components/ui/Switch'
import { Text } from '@/components/ui/Text'
import { Textarea } from '@/components/ui/Textarea'
import { KeypointsEditor } from '@/components/KeypointsEditor'
import { keypointsBody, keypointsComplete, toKeypointsDraft } from '@/components/keypointsDraft'
import { LabelService, type LabelType } from '@/gen/krill/v1/label_pb'
import { invalidateService } from '@/lib/queryClient'
import { exportClasses, palette } from '@/lib/labels'
import { labelClient } from '@/lib/clients'
import { flash } from '@/lib/flash'
import { plural } from '@/lib/format'
import { putFile } from '@/lib/upload'

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
  const [title, setTitle] = useState(type?.title ?? '')
  const [color, setColor] = useState(type?.color ?? palette[0])
  const [description, setDescription] = useState(type?.description ?? '')
  const [guideline, setGuideline] = useState(type?.guideline ?? '')
  const [oriented, setOriented] = useState(type?.oriented ?? false)
  const [keypoints, setKeypoints] = useState(() => toKeypointsDraft(type))
  const [attributes, setAttributes] = useState<AttributeDraft[]>(() => toDrafts(type))

  const onSuccess = async () => {
    await invalidateService(LabelService)
    onClose()
  }
  const create = useMutation(LabelService.method.createLabelType, { onSuccess })
  const update = useMutation(LabelService.method.updateLabelType, { onSuccess })
  const busy = create.isPending || update.isPending

  const parsed = attributes
    .filter((a) => a.name.trim())
    .map((a) => ({ name: a.name.trim().toLowerCase(), options: parseOptions(a.options) }))
  const classes = exportClasses(name.trim().toLowerCase() || 'name', parsed)

  function submit(e: React.FormEvent) {
    e.preventDefault()
    const body = {
      name,
      title,
      color,
      description,
      guideline,
      oriented,
      attributes: parsed,
      ...keypointsBody(keypoints),
    }
    if (type) update.mutate({ id: type.id, ...body })
    else create.mutate(body)
  }

  const patch = (id: number, p: Partial<AttributeDraft>) =>
    setAttributes((all) => all.map((a) => (a.id === id ? { ...a, ...p } : a)))

  return (
    <form onSubmit={submit}>
      <DialogTitle>{type ? `Edit ${type.name}` : 'New label type'}</DialogTitle>
      <DialogBody>
        <FieldGroup>
          <Field>
            <Label>Name</Label>
            <Description>Lowercase snake_case, such as torpedo_hole.</Description>
            <Input
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="torpedo_hole"
              required
              maxLength={40}
              autoComplete="off"
            />
          </Field>

          <Field>
            <Label>Title</Label>
            <Input
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              placeholder="Torpedo hole"
              maxLength={60}
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
            <Description>What counts as this object.</Description>
            <Textarea
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              rows={2}
            />
          </Field>

          <Field>
            <Label>Guideline</Label>
            <Textarea
              value={guideline}
              onChange={(e) => setGuideline(e.target.value)}
              rows={4}
              maxLength={4000}
            />
          </Field>

          <Field>
            <Label>Oriented</Label>
            <Description>Rotated boxes, exported as YOLO-OBB.</Description>
            <div data-slot="control">
              <Switch
                checked={oriented}
                onChange={setOriented}
                disabled={keypoints.keypoints.length > 0}
              />
            </div>
          </Field>

          {!oriented && <KeypointsEditor draft={keypoints} onChange={setKeypoints} />}

          {type && (type.oriented || hasReferenceQuad(type)) && <Reference type={type} />}

          {type && <Examples type={type} />}

          <Fieldset>
            <Legend>Attributes</Legend>
            <Text>Options are comma separated.</Text>
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

          <FormError error={create.error ?? update.error} />
        </FieldGroup>
      </DialogBody>
      <DialogActions>
        <Button plain onClick={onClose}>
          Cancel
        </Button>
        <Button
          type="submit"
          color="sky"
          disabled={busy || !name.trim() || !keypointsComplete(keypoints)}
        >
          {type ? 'Save' : 'Create'}
        </Button>
      </DialogActions>
    </form>
  )
}

function Examples({ type }: { type: LabelType }) {
  const [caption, setCaption] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<unknown>(null)
  const input = useRef<HTMLInputElement>(null)

  async function upload(file: File) {
    setBusy(true)
    setError(null)
    try {
      const { key, uploadUrl } = await labelClient.createLabelExampleUpload({
        labelTypeId: type.id,
      })
      await putFile(uploadUrl, file, () => {}, new AbortController().signal)
      await labelClient.addLabelExample({ labelTypeId: type.id, key, caption })
      setCaption('')
      await invalidateService(LabelService)
    } catch (err) {
      setError(err)
    } finally {
      setBusy(false)
    }
  }

  async function remove(id: bigint) {
    setError(null)
    try {
      await labelClient.deleteLabelExample({ id })
      await invalidateService(LabelService)
    } catch (err) {
      setError(err)
    }
  }

  return (
    <Fieldset>
      <Legend>Examples</Legend>
      {type.examples.length > 0 && (
        <ul className="mt-3 grid grid-cols-3 gap-2">
          {type.examples.map((e) => (
            <li key={String(e.id)} className="group relative">
              <img
                src={e.url}
                alt={e.caption}
                className="aspect-video w-full rounded-md bg-zinc-950 object-contain"
              />
              <p className="mt-1 truncate text-xs text-zinc-500 dark:text-zinc-400">{e.caption}</p>
              <button
                type="button"
                aria-label="Remove example"
                onClick={() => void remove(e.id)}
                className="absolute top-1 right-1 rounded-full bg-zinc-950/60 p-0.5 text-white opacity-0 group-hover:opacity-100 hover:bg-zinc-950/80 focus:opacity-100"
              >
                <XMarkIcon className="size-4" />
              </button>
            </li>
          ))}
        </ul>
      )}
      <div className="mt-3 flex gap-2">
        <Input
          aria-label="Caption"
          placeholder="Caption"
          value={caption}
          onChange={(e) => setCaption(e.target.value)}
          maxLength={200}
        />
        <Button outline disabled={busy} onClick={() => input.current?.click()}>
          <PhotoIcon data-slot="icon" />
          {busy ? 'Uploading…' : 'Add image'}
        </Button>
        <input
          ref={input}
          type="file"
          accept="image/*"
          hidden
          onChange={(e) => {
            const file = e.target.files?.[0]
            e.target.value = ''
            if (file) void upload(file)
          }}
        />
      </div>
      <FormError error={error} className="mt-3" />
    </Fieldset>
  )
}

function hasReferenceQuad(type: LabelType) {
  return type.quads.some((q) => q.reference)
}

function Reference({ type }: { type: LabelType }) {
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<unknown>(null)
  const input = useRef<HTMLInputElement>(null)
  const fitted = {
    onSuccess: async (res: { boxCount: number }) => {
      flash.success(`Fitting ${plural(res.boxCount, 'box', 'boxes')}`)
      await invalidateService(LabelService)
    },
    onError: setError,
  }
  const fit = useMutation(LabelService.method.fitRotations, fitted)
  const fitKeypoints = useMutation(LabelService.method.fitKeypoints, fitted)

  async function setReference(file: File | null) {
    setBusy(true)
    setError(null)
    try {
      let key = ''
      if (file) {
        const upload = await labelClient.createLabelExampleUpload({ labelTypeId: type.id })
        await putFile(upload.uploadUrl, file, () => {}, new AbortController().signal)
        key = upload.key
      }
      await labelClient.setLabelReference({ labelTypeId: type.id, key })
      await invalidateService(LabelService)
    } catch (err) {
      setError(err)
    } finally {
      setBusy(false)
    }
  }

  return (
    <Fieldset>
      <Legend>Reference design</Legend>
      <div className="mt-3 flex items-start gap-3">
        {type.referenceUrl && (
          <div className="group relative w-40 shrink-0">
            <img
              src={type.referenceUrl}
              alt="Reference design"
              className="aspect-video w-full rounded-md bg-zinc-950 object-contain"
            />
            <button
              type="button"
              aria-label="Remove reference design"
              disabled={busy}
              onClick={() => void setReference(null)}
              className="absolute top-1 right-1 rounded-full bg-zinc-950/60 p-0.5 text-white opacity-0 group-hover:opacity-100 hover:bg-zinc-950/80 focus:opacity-100"
            >
              <XMarkIcon className="size-4" />
            </button>
          </div>
        )}
        <Button outline disabled={busy} onClick={() => input.current?.click()}>
          <PhotoIcon data-slot="icon" />
          {busy ? 'Uploading…' : type.referenceUrl ? 'Replace' : 'Add image'}
        </Button>
        <input
          ref={input}
          type="file"
          accept="image/png,image/jpeg,image/webp"
          hidden
          onChange={(e) => {
            const file = e.target.files?.[0]
            e.target.value = ''
            if (file) void setReference(file)
          }}
        />
      </div>
      {type.oriented && type.unrotatedBoxCount > 0 && (
        <div className="mt-4 flex items-center gap-3">
          <Button
            outline
            disabled={fit.isPending}
            onClick={() => {
              setError(null)
              fit.mutate({ labelTypeId: type.id })
            }}
          >
            <ArrowPathIcon data-slot="icon" />
            Fit rotations
          </Button>
          <Text>{plural(type.unrotatedBoxCount, 'unrotated box', 'unrotated boxes')}</Text>
        </div>
      )}
      {hasReferenceQuad(type) && type.referenceUrl && type.unposedBoxCount > 0 && (
        <div className="mt-4 flex items-center gap-3">
          <Button
            outline
            disabled={fitKeypoints.isPending}
            onClick={() => {
              setError(null)
              fitKeypoints.mutate({ labelTypeId: type.id })
            }}
          >
            <ArrowPathIcon data-slot="icon" />
            Fit keypoints
          </Button>
          <Text>
            {plural(type.unposedBoxCount, 'box without keypoints', 'boxes without keypoints')}
          </Text>
        </div>
      )}
      <FormError error={error} className="mt-3" />
    </Fieldset>
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
