import { ConnectError } from '@connectrpc/connect'
import { useMutation, useQuery } from '@connectrpc/connect-query'
import { EnvelopeIcon, LockClosedIcon } from '@heroicons/react/16/solid'
import { AnimatePresence, motion } from 'motion/react'
import { useMemo, useState } from 'react'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { Callout, QueryError } from '@/components/ui/Callout'
import {
  DescriptionDetails,
  DescriptionList,
  DescriptionTerm,
} from '@/components/ui/DescriptionList'
import { Description, Field, FieldGroup, LabelRow } from '@/components/ui/Fieldset'
import { Heading, Subheading } from '@/components/ui/Heading'
import { Input } from '@/components/ui/Input'
import LoadingSpinner from '@/components/ui/LoadingSpinner'
import PageContentBlock from '@/components/ui/PageContentBlock'
import { Select } from '@/components/ui/Select'
import { Switch } from '@/components/ui/Switch'
import { Code, TextButton } from '@/components/ui/Text'
import {
  AdminService,
  type Setting,
  SettingErrorSchema,
  SettingKind,
  SettingSource,
} from '@/gen/krill/v1/admin_pb'
import { refreshSession, useUser } from '@/lib/auth'
import { flash } from '@/lib/flash'
import { plural } from '@/lib/format'
import { invalidateService } from '@/lib/queryClient'
import { SlackCheck } from './SlackCheck'

// An edit is a new value, or dropping the saved one for the default.
type Edit = { value: string } | { useDefault: true }

function isTrue(v: string) {
  return ['1', 't', 'true'].includes(v.toLowerCase())
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section className="grid gap-x-8 gap-y-6 border-t border-zinc-950/5 py-10 sm:grid-cols-3 dark:border-white/5">
      <Subheading>{title}</Subheading>
      <div className="sm:col-span-2">{children}</div>
    </section>
  )
}

function SettingField({
  setting: s,
  edit,
  error,
  onEdit,
}: {
  setting: Setting
  edit: Edit | undefined
  error?: string
  onEdit: (e: Edit | undefined) => void
}) {
  const locked = s.source === SettingSource.ENVIRONMENT
  const secret = s.kind === SettingKind.SECRET
  const usingDefault = edit !== undefined && 'useDefault' in edit
  const value =
    edit && 'value' in edit ? edit.value : usingDefault ? s.defaultValue : secret ? '' : s.value
  const set = (v: string) => onEdit(!secret && v === s.value ? undefined : { value: v })
  const saved = s.source === SettingSource.SAVED && !usingDefault

  let action: React.ReactNode = null
  if (locked) {
    action = (
      <Badge>
        <LockClosedIcon className="size-3" />
        Environment
      </Badge>
    )
  } else if (edit) {
    action = <TextButton onClick={() => onEdit(undefined)}>Undo</TextButton>
  } else if (saved) {
    action = (
      <TextButton onClick={() => onEdit({ useDefault: true })}>
        {secret || !s.defaultValue ? 'Clear' : 'Use default'}
      </TextButton>
    )
  }

  let control: React.ReactNode
  switch (s.kind) {
    case SettingKind.BOOL:
      control = (
        <div data-slot="control">
          <Switch
            checked={isTrue(value)}
            onChange={(on) => set(on ? 'true' : 'false')}
            disabled={locked}
          />
        </div>
      )
      break
    case SettingKind.CHOICE:
      control = (
        <Select value={value} onChange={(e) => set(e.target.value)} disabled={locked}>
          {s.choices.map((c) => (
            <option key={c.value} value={c.value}>
              {c.label}
            </option>
          ))}
        </Select>
      )
      break
    default:
      control = (
        <Input
          name={s.key}
          type={secret ? 'password' : s.kind === SettingKind.STRING ? 'text' : 'number'}
          step={s.kind === SettingKind.FLOAT ? 'any' : undefined}
          autoComplete="off"
          spellCheck={false}
          value={value}
          placeholder={
            secret
              ? s.isSet && !usingDefault
                ? '••••••••••••'
                : ''
              : s.defaultValue && !locked
                ? s.defaultValue
                : undefined
          }
          onChange={(e) => set(e.target.value)}
          disabled={locked}
        />
      )
  }

  return (
    <Field disabled={locked} error={error}>
      <LabelRow action={action}>{s.label}</LabelRow>
      {s.description && <Description>{s.description}</Description>}
      {control}
      {s.key === 'KRILL_EMAIL_VERIFICATION' && value === 'verify' && s.value !== 'verify' && (
        <VerifyWarning />
      )}
    </Field>
  )
}

// Accounts made under the other modes never entered a code, so turning codes
// on stops them, possibly including the admin saving this, until they do.
function VerifyWarning() {
  const me = useUser()
  return (
    <Callout type="warning" className="mt-3">
      Accounts without a verified email{me && !me.emailVerified ? ', including yours,' : ''} will
      need to enter an emailed code before they can continue.
    </Callout>
  )
}

function TestEmailButton({ disabled }: { disabled: boolean }) {
  const send = useMutation(AdminService.method.sendTestEmail, {
    onSuccess: (res) =>
      res.logged
        ? flash.info('Test email written to the API log')
        : flash.success('Test email sent', `Check ${res.to}.`),
    onError: (err) => flash.error('Could not send a test email', err),
  })
  return (
    <Button
      outline
      onClick={() => send.mutate({})}
      disabled={disabled || send.isPending}
      title={disabled ? 'Save your changes first' : undefined}
    >
      <EnvelopeIcon data-slot="icon" />
      Send test email
    </Button>
  )
}

function EnvOnly({ settings }: { settings: Setting[] }) {
  return (
    <DescriptionList>
      {settings.map((s) => (
        <div key={s.key} className="contents">
          <DescriptionTerm>
            {s.label}
            <span className="block font-mono text-xs/5">{s.key}</span>
          </DescriptionTerm>
          <DescriptionDetails className="break-all">
            {s.kind === SettingKind.SECRET ? (
              s.isSet ? (
                'Set'
              ) : (
                <span className="text-zinc-500 dark:text-zinc-400">Not set</span>
              )
            ) : s.value ? (
              <Code>{s.value}</Code>
            ) : (
              <span className="text-zinc-500 dark:text-zinc-400">Not set</span>
            )}
          </DescriptionDetails>
        </div>
      ))}
    </DescriptionList>
  )
}

function omit<T>(obj: Record<string, T>, key: string): Record<string, T> {
  const out = { ...obj }
  delete out[key]
  return out
}

function groupBy(settings: Setting[]) {
  const groups = new Map<string, Setting[]>()
  for (const s of settings) groups.set(s.group, [...(groups.get(s.group) ?? []), s])
  return [...groups]
}

export function SettingsPage() {
  const { data, isPending, error } = useQuery(AdminService.method.listSettings, {})
  const [edits, setEdits] = useState<Record<string, Edit>>({})
  const [errors, setErrors] = useState<Record<string, string>>({})

  const settings = useMemo(() => data?.settings ?? [], [data])
  const editable = useMemo(() => groupBy(settings.filter((s) => !s.envOnly)), [settings])
  const envOnly = useMemo(() => settings.filter((s) => s.envOnly), [settings])
  const count = Object.keys(edits).length

  const save = useMutation(AdminService.method.updateSettings, {
    onSuccess: async () => {
      setEdits({})
      setErrors({})
      flash.success('Settings saved')
      await Promise.all([invalidateService(AdminService), refreshSession()])
    },
    onError: (err) => {
      const detail = err instanceof ConnectError ? err.findDetails(SettingErrorSchema)[0] : null
      const s = detail && settings.find((x) => x.key === detail.key)
      if (detail && s) {
        setErrors({ [detail.key]: `${s.label} ${detail.message}.` })
        document.getElementsByName(detail.key)[0]?.scrollIntoView({ block: 'center' })
      } else {
        flash.error('Could not save settings', err)
      }
    },
  })

  const onEdit = (key: string) => (e: Edit | undefined) => {
    setEdits((all) => (e ? { ...all, [key]: e } : omit(all, key)))
    setErrors((all) => omit(all, key))
  }
  const submit = () =>
    save.mutate({
      changes: Object.entries(edits).map(([key, e]) =>
        'useDefault' in e ? { key, useDefault: true } : { key, value: e.value },
      ),
    })

  return (
    <PageContentBlock title="Settings · Admin · Krill">
      <Heading>Settings</Heading>
      {isPending ? (
        <div className="flex justify-center py-16">
          <LoadingSpinner />
        </div>
      ) : error ? (
        <QueryError title="Could not load settings" error={error} className="mt-8" />
      ) : (
        <form
          className="mt-8"
          onSubmit={(e) => {
            e.preventDefault()
            submit()
          }}
        >
          {editable.map(([group, items]) => (
            <Section key={group} title={group}>
              <FieldGroup>
                {items.map((s) => (
                  <SettingField
                    key={s.key}
                    setting={s}
                    edit={edits[s.key]}
                    error={errors[s.key]}
                    onEdit={onEdit(s.key)}
                  />
                ))}
                {group === 'Email' && (
                  <div className="flex flex-wrap gap-3">
                    <TestEmailButton disabled={items.some((s) => s.key in edits)} />
                    <Button plain to="/admin/emails">
                      Preview emails
                    </Button>
                  </div>
                )}
                {group === 'Slack' && <SlackCheck disabled={items.some((s) => s.key in edits)} />}
              </FieldGroup>
            </Section>
          ))}
          {envOnly.length > 0 && (
            <Section title="Environment only">
              <EnvOnly settings={envOnly} />
            </Section>
          )}

          <AnimatePresence>
            {count > 0 && (
              <motion.div
                initial={{ opacity: 0, y: 16 }}
                animate={{ opacity: 1, y: 0 }}
                exit={{ opacity: 0, y: 16 }}
                transition={{ duration: 0.15 }}
                className="sticky bottom-4 mt-6 flex items-center justify-between gap-4 rounded-xl bg-white px-4 py-3 shadow-lg ring-1 ring-zinc-950/10 dark:bg-zinc-800 dark:ring-white/10"
              >
                <p className="text-sm/6 text-zinc-700 dark:text-zinc-300">
                  {plural(count, 'unsaved change')}
                </p>
                <div className="flex gap-3">
                  <Button
                    plain
                    onClick={() => {
                      setEdits({})
                      setErrors({})
                    }}
                  >
                    Discard
                  </Button>
                  <Button type="submit" color="sky" disabled={save.isPending}>
                    Save
                  </Button>
                </div>
              </motion.div>
            )}
          </AnimatePresence>
        </form>
      )}
    </PageContentBlock>
  )
}
