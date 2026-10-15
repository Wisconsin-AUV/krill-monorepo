import { timestampDate, type Timestamp } from '@bufbuild/protobuf/wkt'
import { useMutation, useQuery } from '@connectrpc/connect-query'
import { CheckIcon, ClipboardIcon, PlusIcon } from '@heroicons/react/20/solid'
import { useState } from 'react'
import { useSearchParams } from 'react-router'
import { SegmentedTabs } from '@/components/SegmentedTabs'
import { Avatar } from '@/components/ui/Avatar'
import { Badge, type BadgeColor } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { FormError, QueryError } from '@/components/ui/Callout'
import { Dialog, DialogActions, DialogBody, DialogTitle } from '@/components/ui/Dialog'
import { Description, Field, FieldGroup, Label } from '@/components/ui/Fieldset'
import { Heading } from '@/components/ui/Heading'
import { Input, InputGroup } from '@/components/ui/Input'
import LoadingSpinner from '@/components/ui/LoadingSpinner'
import PageContentBlock from '@/components/ui/PageContentBlock'
import { Select } from '@/components/ui/Select'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/Table'
import { Textarea } from '@/components/ui/Textarea'
import {
  InviteService,
  InviteState,
  type CreateInviteResponse,
  type Invite,
} from '@/gen/krill/v1/invite_pb'
import type { Profile } from '@/gen/krill/v1/stats_pb'
import { Role } from '@/gen/krill/v1/user_pb'
import { useRoleLabel, useRoles, useSession } from '@/lib/auth'
import { flash } from '@/lib/flash'
import { formatRelative } from '@/lib/format'
import { invalidateService } from '@/lib/queryClient'

const fullDate = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' })

const tableClass = 'mt-8 [--gutter:--spacing(6)] lg:[--gutter:--spacing(10)]'

const expiries = [
  { hours: 24, label: '1 day' },
  { hours: 24 * 7, label: '7 days' },
  { hours: 24 * 30, label: '30 days' },
  { hours: 24 * 90, label: '90 days' },
]

const states: Record<InviteState, [string, BadgeColor]> = {
  [InviteState.UNSPECIFIED]: ['', 'zinc'],
  [InviteState.ACTIVE]: ['Active', 'emerald'],
  [InviteState.EXPIRED]: ['Expired', 'zinc'],
  [InviteState.REVOKED]: ['Revoked', 'red'],
  [InviteState.USED_UP]: ['Used up', 'zinc'],
}

function When({ at }: { at?: Timestamp }) {
  if (!at) return null
  const date = timestampDate(at)
  return <span title={fullDate.format(date)}>{formatRelative(date)}</span>
}

function Person({ profile }: { profile?: Profile }) {
  if (!profile) return <span className="text-zinc-500 dark:text-zinc-400">—</span>
  return (
    <span className="flex items-center gap-2">
      <Avatar seed={profile.id} className="size-6" />
      <span className="text-zinc-950 dark:text-white">{profile.name}</span>
    </span>
  )
}

function CopyButton({ text }: { text: string }) {
  const [copied, setCopied] = useState(false)
  return (
    <Button
      outline
      onClick={async () => {
        await navigator.clipboard.writeText(text)
        setCopied(true)
        setTimeout(() => setCopied(false), 1500)
      }}
    >
      {copied ? <CheckIcon data-slot="icon" /> : <ClipboardIcon data-slot="icon" />}
      {copied ? 'Copied' : 'Copy'}
    </Button>
  )
}

function RoleOptions() {
  const roles = useRoles()
  return roles.map((r) => (
    <option key={r.role} value={r.role}>
      {r.label}
    </option>
  ))
}

function NewInviteDialog({ open, onClose }: { open: boolean; onClose: () => void }) {
  const canEmail = useSession().data?.emailCodesEnabled ?? false
  const blank = { role: Role.LABELER as Role, maxUses: 1, hours: 24 * 7, note: '', email: '' }
  const [form, setForm] = useState(blank)
  const [created, setCreated] = useState<CreateInviteResponse | null>(null)
  const create = useMutation(InviteService.method.createInvite, {
    onSuccess: async (res) => {
      setCreated(res)
      await invalidateService(InviteService)
    },
  })
  const close = () => {
    onClose()
    create.reset()
    setCreated(null)
    setForm(blank)
  }

  if (created) {
    return (
      <Dialog open={open} onClose={close} size="lg">
        <DialogTitle>{created.invite?.email ? `Invite sent` : 'Invite link'}</DialogTitle>
        <DialogBody>
          <Field>
            <Label>Link</Label>
            <Description>Shown once.</Description>
            <div className="mt-3 flex gap-2">
              <Input readOnly value={created.url} onFocus={(e) => e.target.select()} />
              <CopyButton text={created.url} />
            </div>
          </Field>
        </DialogBody>
        <DialogActions>
          <Button color="sky" onClick={close}>
            Done
          </Button>
        </DialogActions>
      </Dialog>
    )
  }

  return (
    <Dialog open={open} onClose={close} size="lg">
      <form
        onSubmit={(e) => {
          e.preventDefault()
          create.mutate({
            role: form.role,
            maxUses: form.maxUses,
            expiresInHours: form.hours,
            note: form.note,
            email: form.email,
          })
        }}
      >
        <DialogTitle>New invite</DialogTitle>
        <DialogBody>
          <FieldGroup>
            <div className="grid grid-cols-1 gap-6 sm:grid-cols-3">
              <Field>
                <Label>Role</Label>
                <Select
                  value={form.role}
                  onChange={(e) => setForm((f) => ({ ...f, role: Number(e.target.value) as Role }))}
                >
                  <RoleOptions />
                </Select>
              </Field>
              <Field>
                <Label>Uses</Label>
                <Input
                  type="number"
                  min={1}
                  max={100}
                  required
                  value={form.maxUses}
                  onChange={(e) => setForm((f) => ({ ...f, maxUses: Number(e.target.value) }))}
                />
              </Field>
              <Field>
                <Label>Expires in</Label>
                <Select
                  value={form.hours}
                  onChange={(e) => setForm((f) => ({ ...f, hours: Number(e.target.value) }))}
                >
                  {expiries.map((x) => (
                    <option key={x.hours} value={x.hours}>
                      {x.label}
                    </option>
                  ))}
                </Select>
              </Field>
            </div>
            <Field>
              <Label>Note</Label>
              <Input
                value={form.note}
                maxLength={200}
                placeholder="Fall recruits"
                onChange={(e) => setForm((f) => ({ ...f, note: e.target.value }))}
              />
            </Field>
            <Field disabled={!canEmail}>
              <Label>Email the link to</Label>
              {!canEmail && <Description>Email is off.</Description>}
              <Input
                type="email"
                value={form.email}
                placeholder="Optional"
                onChange={(e) => setForm((f) => ({ ...f, email: e.target.value }))}
              />
            </Field>
            <FormError error={create.error} />
          </FieldGroup>
        </DialogBody>
        <DialogActions>
          <Button plain onClick={close}>
            Cancel
          </Button>
          <Button type="submit" color="sky" disabled={create.isPending}>
            {form.email ? 'Create and send' : 'Create link'}
          </Button>
        </DialogActions>
      </form>
    </Dialog>
  )
}

function InviteRow({ invite }: { invite: Invite }) {
  const roleLabel = useRoleLabel()
  const revoke = useMutation(InviteService.method.revokeInvite, {
    onSuccess: () => invalidateService(InviteService),
    onError: (err) => flash.error('Could not revoke invite', err),
  })
  const [label, color] = states[invite.state]
  return (
    <TableRow>
      <TableCell>
        <div className="font-medium text-zinc-950 dark:text-white">
          {invite.note || invite.email || `Invite ${invite.id}`}
        </div>
        {invite.note && invite.email && (
          <div className="text-zinc-500 dark:text-zinc-400">{invite.email}</div>
        )}
      </TableCell>
      <TableCell>{roleLabel(invite.role)}</TableCell>
      <TableCell>
        <Badge color={color}>{label}</Badge>
      </TableCell>
      <TableCell className="tabular-nums">
        {invite.uses} / {invite.maxUses}
      </TableCell>
      <TableCell>
        <div className="flex max-w-56 flex-wrap gap-x-3 gap-y-1">
          {invite.usedBy.length === 0 ? (
            <span className="text-zinc-500 dark:text-zinc-400">—</span>
          ) : (
            invite.usedBy.map((u) => <Person key={u.user?.id} profile={u.user} />)
          )}
        </div>
      </TableCell>
      <TableCell className="text-zinc-500 dark:text-zinc-400">
        <When at={invite.expiresAt} />
      </TableCell>
      <TableCell>
        <Person profile={invite.createdBy} />
      </TableCell>
      <TableCell className="text-right">
        {invite.state === InviteState.ACTIVE && (
          <Button
            plain
            disabled={revoke.isPending}
            onClick={() => revoke.mutate({ id: invite.id })}
          >
            Revoke
          </Button>
        )}
      </TableCell>
    </TableRow>
  )
}

function Invites() {
  const { data, isPending, error } = useQuery(InviteService.method.listInvites, {})
  const [showClosed, setShowClosed] = useState(false)
  if (isPending) {
    return (
      <div className="flex justify-center py-16">
        <LoadingSpinner />
      </div>
    )
  }
  if (error) return <QueryError title="Could not load invites" error={error} className="mt-8" />
  const active = data.invites.filter((i) => i.state === InviteState.ACTIVE)
  const shown = showClosed ? data.invites : active
  const closed = data.invites.length - active.length

  return (
    <>
      {shown.length === 0 ? (
        <p className="py-10 text-center text-sm/6 text-zinc-500 dark:text-zinc-400">
          No active invites
        </p>
      ) : (
        <Table className={tableClass}>
          <TableHead>
            <TableRow>
              <TableHeader>Invite</TableHeader>
              <TableHeader>Role</TableHeader>
              <TableHeader>Status</TableHeader>
              <TableHeader>Uses</TableHeader>
              <TableHeader>Used by</TableHeader>
              <TableHeader>Expires</TableHeader>
              <TableHeader>Created by</TableHeader>
              <TableHeader className="relative w-0">
                <span className="sr-only">Actions</span>
              </TableHeader>
            </TableRow>
          </TableHead>
          <TableBody>
            {shown.map((i) => (
              <InviteRow key={String(i.id)} invite={i} />
            ))}
          </TableBody>
        </Table>
      )}
      {closed > 0 && (
        <div className="mt-6 flex justify-center">
          <Button plain onClick={() => setShowClosed((s) => !s)}>
            {showClosed ? 'Hide closed' : `Show ${closed} closed`}
          </Button>
        </div>
      )}
    </>
  )
}

function parseEmails(text: string) {
  return text
    .split(/[\s,;]+/)
    .map((e) => e.trim())
    .filter(Boolean)
}

function ApprovedEmails() {
  const roleLabel = useRoleLabel()
  const { data, isPending, error } = useQuery(InviteService.method.listApprovedEmails, {})
  const [text, setText] = useState('')
  const [role, setRole] = useState<Role>(Role.LABELER)
  const [filter, setFilter] = useState('')
  const add = useMutation(InviteService.method.addApprovedEmails, {
    onSuccess: async (res) => {
      setText('')
      await invalidateService(InviteService)
      flash.success(
        res.approvedEmails.length === 1
          ? 'Email approved'
          : `${res.approvedEmails.length} emails approved`,
      )
    },
  })
  const remove = useMutation(InviteService.method.removeApprovedEmail, {
    onSuccess: () => invalidateService(InviteService),
    onError: (err) => flash.error('Could not remove email', err),
  })
  const emails = parseEmails(text)
  const query = filter.trim().toLowerCase()
  const rows = (data?.approvedEmails ?? []).filter((a) => !query || a.email.includes(query))

  return (
    <>
      <form
        className="mt-8 grid grid-cols-1 items-start gap-4 sm:grid-cols-[1fr_10rem_auto]"
        onSubmit={(e) => {
          e.preventDefault()
          add.mutate({ emails, role })
        }}
      >
        <Field>
          <Label className="sr-only">Emails</Label>
          <Textarea
            rows={2}
            placeholder="bucky@wisc.edu, …"
            value={text}
            onChange={(e) => setText(e.target.value)}
          />
        </Field>
        <Field>
          <Label className="sr-only">Role</Label>
          <Select value={role} onChange={(e) => setRole(Number(e.target.value) as Role)}>
            <RoleOptions />
          </Select>
        </Field>
        <Button type="submit" color="sky" disabled={emails.length === 0 || add.isPending}>
          Approve
        </Button>
      </form>
      <FormError error={add.error} className="mt-4" />

      {isPending ? (
        <div className="flex justify-center py-16">
          <LoadingSpinner />
        </div>
      ) : error ? (
        <QueryError title="Could not load approved emails" error={error} className="mt-8" />
      ) : data.approvedEmails.length === 0 ? (
        <p className="py-10 text-center text-sm/6 text-zinc-500 dark:text-zinc-400">
          No approved emails
        </p>
      ) : (
        <>
          {data.approvedEmails.length > 10 && (
            <div className="mt-8 sm:max-w-xs">
              <InputGroup>
                <Input
                  type="search"
                  aria-label="Search approved emails"
                  placeholder="Search"
                  value={filter}
                  onChange={(e) => setFilter(e.target.value)}
                />
              </InputGroup>
            </div>
          )}
          <Table className={tableClass}>
            <TableHead>
              <TableRow>
                <TableHeader>Email</TableHeader>
                <TableHeader>Role</TableHeader>
                <TableHeader>Status</TableHeader>
                <TableHeader>Approved by</TableHeader>
                <TableHeader className="relative w-0">
                  <span className="sr-only">Actions</span>
                </TableHeader>
              </TableRow>
            </TableHead>
            <TableBody>
              {rows.map((a) => (
                <TableRow key={a.email}>
                  <TableCell className="font-medium text-zinc-950 dark:text-white">
                    {a.email}
                  </TableCell>
                  <TableCell>{roleLabel(a.role)}</TableCell>
                  <TableCell>
                    {a.usedAt ? (
                      <span className="flex items-center gap-2">
                        <Badge color="emerald">Signed up</Badge>
                        {a.usedBy && <Person profile={a.usedBy} />}
                      </span>
                    ) : (
                      <Badge color="amber">Waiting</Badge>
                    )}
                  </TableCell>
                  <TableCell>
                    <span className="flex items-center gap-2">
                      <Person profile={a.createdBy} />
                      <span className="text-zinc-500 dark:text-zinc-400">
                        <When at={a.createdAt} />
                      </span>
                    </span>
                  </TableCell>
                  <TableCell className="text-right">
                    <Button
                      plain
                      disabled={remove.isPending}
                      onClick={() => remove.mutate({ email: a.email })}
                    >
                      Remove
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </>
      )}
    </>
  )
}

const tabs = [
  { key: 'links', label: 'Links' },
  { key: 'emails', label: 'Pre-approved emails' },
] as const

export function InvitesPage() {
  const [params, setParams] = useSearchParams()
  const tab = params.get('tab') === 'emails' ? 'emails' : 'links'
  const [creating, setCreating] = useState(false)

  return (
    <PageContentBlock title="Invites · Admin · Krill">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <Heading>Invites</Heading>
        <div className="flex items-center gap-4">
          <SegmentedTabs
            options={tabs}
            value={tab}
            onChange={(key) => setParams(key === 'links' ? {} : { tab: key }, { replace: true })}
          />
          {tab === 'links' && (
            <Button color="sky" onClick={() => setCreating(true)}>
              <PlusIcon data-slot="icon" />
              New invite
            </Button>
          )}
        </div>
      </div>
      {tab === 'links' ? <Invites /> : <ApprovedEmails />}
      <NewInviteDialog open={creating} onClose={() => setCreating(false)} />
    </PageContentBlock>
  )
}
