import { useMutation, useQuery } from '@connectrpc/connect-query'
import { ArrowsUpDownIcon } from '@heroicons/react/20/solid'
import { useState } from 'react'
import { useNavigate } from 'react-router'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { FormError, QueryError } from '@/components/ui/Callout'
import { Dialog, DialogActions, DialogBody, DialogTitle } from '@/components/ui/Dialog'
import { Field, Label } from '@/components/ui/Fieldset'
import LoadingSpinner from '@/components/ui/LoadingSpinner'
import { Select } from '@/components/ui/Select'
import { UserService, type User } from '@/gen/krill/v1/user_pb'
import { useRoleLabel, useUser } from '@/lib/auth'
import { flash } from '@/lib/flash'
import { formatNumber } from '@/lib/format'
import { invalidateService } from '@/lib/queryClient'

function UserSelect({
  label,
  users,
  value,
  onChange,
  exclude,
}: {
  label: string
  users: User[]
  value: string
  onChange: (id: string) => void
  exclude?: string
}) {
  return (
    <Field>
      <Label>{label}</Label>
      <Select value={value} onChange={(e) => onChange(e.target.value)}>
        <option value="" disabled>
          Choose an account
        </option>
        {users
          .filter((u) => u.id !== exclude)
          .map((u) => (
            <option key={u.id} value={u.id}>
              {u.name} · @{u.username} · {u.email}
            </option>
          ))}
      </Select>
    </Field>
  )
}

function Methods({ user }: { user: User }) {
  return (
    <span className="flex flex-wrap gap-1.5">
      {user.hasPassword && <Badge>Password</Badge>}
      {user.slackLinked && <Badge>Slack</Badge>}
      {!user.hasPassword && !user.slackLinked && <Badge color="zinc">None</Badge>}
    </span>
  )
}

function Preview({ sourceId, targetId }: { sourceId: string; targetId: string }) {
  const roleLabel = useRoleLabel()
  const { data, isPending, error } = useQuery(UserService.method.previewMergeUsers, {
    sourceId,
    targetId,
  })
  if (isPending) {
    return (
      <div className="flex justify-center py-8">
        <LoadingSpinner />
      </div>
    )
  }
  if (error) return <QueryError title="Could not preview the merge" error={error} />
  const { source, target, result, counts } = data
  if (!source || !target || !result) return null
  const rows: [string, React.ReactNode, React.ReactNode][] = [
    ['Role', roleLabel(target.role), roleLabel(result.role)],
    ['Sign-in', <Methods key="t" user={target} />, <Methods key="r" user={result} />],
    [
      'Email',
      target.email,
      <span key="e" className="flex items-center gap-2">
        {result.email}
        {result.emailVerified && <Badge color="emerald">Verified</Badge>}
      </span>,
    ],
  ]

  return (
    <div className="space-y-6">
      <table className="w-full text-left text-sm/6">
        <thead className="text-zinc-500 dark:text-zinc-400">
          <tr>
            <th className="py-1 pr-4 font-medium">
              <span className="sr-only">Field</span>
            </th>
            <th className="py-1 pr-4 font-medium">{target.name} now</th>
            <th className="py-1 font-medium">After</th>
          </tr>
        </thead>
        <tbody>
          {rows.map(([label, before, after]) => (
            <tr key={label} className="border-t border-zinc-950/5 dark:border-white/5">
              <td className="py-2 pr-4 text-zinc-500 dark:text-zinc-400">{label}</td>
              <td className="py-2 pr-4 text-zinc-950 dark:text-white">{before}</td>
              <td className="py-2 text-zinc-950 dark:text-white">{after}</td>
            </tr>
          ))}
        </tbody>
      </table>

      <div>
        <h3 className="text-sm/6 font-medium text-zinc-950 dark:text-white">
          Moves to {target.name}
        </h3>
        {counts.length === 0 ? (
          <p className="mt-1 text-sm/6 text-zinc-500 dark:text-zinc-400">Nothing</p>
        ) : (
          <ul className="mt-2 divide-y divide-zinc-950/5 text-sm/6 dark:divide-white/5">
            {counts.map((c) => (
              <li key={c.label} className="flex justify-between gap-4 py-1.5">
                <span className="text-zinc-600 dark:text-zinc-300">{c.label}</span>
                <span className="text-zinc-950 tabular-nums dark:text-white">
                  {formatNumber(c.moved)}
                  {c.dropped > 0n && (
                    <span className="ml-2 text-zinc-500 dark:text-zinc-400">
                      {formatNumber(c.dropped)} duplicate{c.dropped === 1n ? '' : 's'} dropped
                    </span>
                  )}
                </span>
              </li>
            ))}
          </ul>
        )}
      </div>
    </div>
  )
}

export function MergeDialog({
  user,
  users,
  onClose,
}: {
  user: User | null
  users: User[]
  onClose: () => void
}) {
  const me = useUser()
  const navigate = useNavigate()
  const [pair, setPair] = useState({ source: '', target: '' })
  const [opened, setOpened] = useState<User | null>(null)
  if (user !== opened) {
    setOpened(user)
    setPair({ source: user?.id ?? '', target: '' })
  }
  const source = users.find((u) => u.id === pair.source)
  const merge = useMutation(UserService.method.mergeUsers, {
    // Invalidating once the dialog has closed keeps the preview from refetching
    // the deleted account and showing an error until the refetches finish.
    onSuccess: (res) => {
      flash.success('Accounts merged', `${source?.name} → ${res.user?.name}`)
      close()
      if (res.user) void navigate(`/admin/users/${res.user.id}`)
      setTimeout(() => void invalidateService(UserService))
    },
  })
  function close() {
    merge.reset()
    onClose()
  }
  const ready = pair.source !== '' && pair.target !== '' && pair.source !== pair.target

  return (
    <Dialog open={user !== null} onClose={close} size="2xl">
      <DialogTitle>Merge accounts</DialogTitle>
      <DialogBody className="space-y-6">
        <div className="grid grid-cols-1 items-end gap-4 sm:grid-cols-[1fr_auto_1fr]">
          <UserSelect
            label="Delete"
            users={users.filter((u) => u.id !== me?.id)}
            value={pair.source}
            onChange={(id) => setPair((p) => ({ ...p, source: id }))}
            exclude={pair.target}
          />
          <Button
            plain
            aria-label="Swap"
            disabled={pair.target === me?.id}
            onClick={() => setPair((p) => ({ source: p.target, target: p.source }))}
          >
            <ArrowsUpDownIcon data-slot="icon" className="sm:rotate-90" />
          </Button>
          <UserSelect
            label="Keep"
            users={users}
            value={pair.target}
            onChange={(id) => setPair((p) => ({ ...p, target: id }))}
            exclude={pair.source}
          />
        </div>
        {ready && <Preview sourceId={pair.source} targetId={pair.target} />}
        <FormError error={merge.error} />
      </DialogBody>
      <DialogActions>
        <Button plain onClick={close}>
          Cancel
        </Button>
        <Button
          color="red"
          disabled={!ready || merge.isPending}
          onClick={() => merge.mutate({ sourceId: pair.source, targetId: pair.target })}
        >
          {source ? `Merge and delete ${source.name}` : 'Merge'}
        </Button>
      </DialogActions>
    </Dialog>
  )
}
