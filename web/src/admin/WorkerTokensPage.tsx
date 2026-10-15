import { timestampDate } from '@bufbuild/protobuf/wkt'
import { useMutation, useQuery } from '@connectrpc/connect-query'
import { ClipboardDocumentIcon, KeyIcon, PlusIcon } from '@heroicons/react/20/solid'
import { useState } from 'react'
import { ConfirmAlert } from '@/components/ConfirmAlert'
import { EmptyState } from '@/components/EmptyState'
import { Avatar } from '@/components/ui/Avatar'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { FormError, QueryError } from '@/components/ui/Callout'
import { Dialog, DialogActions, DialogBody, DialogTitle } from '@/components/ui/Dialog'
import { Field, Label } from '@/components/ui/Fieldset'
import { Heading } from '@/components/ui/Heading'
import { Input } from '@/components/ui/Input'
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
import { ComputeService, type WorkerToken } from '@/gen/krill/v1/compute_pb'
import { flash } from '@/lib/flash'
import { formatRelative, plural } from '@/lib/format'
import { invalidateService } from '@/lib/queryClient'

function CreateDialog({ open, onClose }: { open: boolean; onClose: () => void }) {
  const [name, setName] = useState('')
  const [copied, setCopied] = useState(false)
  const create = useMutation(ComputeService.method.createWorkerToken, {
    onSuccess: () => invalidateService(ComputeService),
  })
  const secret = create.data?.secret
  const close = () => {
    onClose()
    setName('')
    setCopied(false)
    create.reset()
  }
  const copy = async () => {
    if (!secret) return
    try {
      await navigator.clipboard.writeText(secret)
      setCopied(true)
    } catch (err) {
      flash.error('Could not copy', err)
    }
  }

  return (
    <Dialog open={open} onClose={secret ? () => {} : close} size="lg">
      {secret ? (
        <>
          <DialogTitle>{create.data?.token?.name}</DialogTitle>
          <DialogBody>
            <Field>
              <Label>KRILL_WORKER_TOKEN</Label>
              <div className="mt-3 flex gap-2">
                <Input
                  readOnly
                  value={secret}
                  className="font-mono"
                  onFocus={(e) => e.target.select()}
                />
                <Button outline onClick={() => void copy()} aria-label="Copy token">
                  <ClipboardDocumentIcon data-slot="icon" />
                  {copied ? 'Copied' : 'Copy'}
                </Button>
              </div>
            </Field>
            <p className="mt-3 text-sm/6 text-zinc-500 dark:text-zinc-400">Shown once</p>
          </DialogBody>
          <DialogActions>
            <Button color="sky" onClick={close}>
              Done
            </Button>
          </DialogActions>
        </>
      ) : (
        <form
          onSubmit={(e) => {
            e.preventDefault()
            create.mutate({ name })
          }}
        >
          <DialogTitle>Create worker token</DialogTitle>
          <DialogBody>
            <Field>
              <Label>Name</Label>
              <Input
                value={name}
                maxLength={64}
                placeholder="gpu-box"
                onChange={(e) => setName(e.target.value)}
              />
            </Field>
            <FormError error={create.error} className="mt-6" />
          </DialogBody>
          <DialogActions>
            <Button plain onClick={close}>
              Cancel
            </Button>
            <Button type="submit" color="sky" disabled={create.isPending || !name.trim()}>
              Create
            </Button>
          </DialogActions>
        </form>
      )}
    </Dialog>
  )
}

function When({ at }: { at: WorkerToken['createdAt'] }) {
  return <>{at ? formatRelative(timestampDate(at)) : 'Never'}</>
}

export function WorkerTokensPage() {
  const { data, isPending, error } = useQuery(
    ComputeService.method.listWorkerTokens,
    {},
    { refetchInterval: 10_000 },
  )
  const [creating, setCreating] = useState(false)
  const [revoking, setRevoking] = useState<WorkerToken | null>(null)
  const revoke = useMutation(ComputeService.method.revokeWorkerToken, {
    onSuccess: async () => {
      await invalidateService(ComputeService)
      setRevoking(null)
    },
  })
  const tokens = data?.tokens ?? []

  return (
    <PageContentBlock title="Worker tokens · Admin · Krill">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <Heading>Worker tokens</Heading>
        <Button color="sky" onClick={() => setCreating(true)}>
          <PlusIcon data-slot="icon" />
          Create token
        </Button>
      </div>

      {isPending ? (
        <div className="flex justify-center py-16">
          <LoadingSpinner />
        </div>
      ) : error ? (
        <QueryError title="Could not load worker tokens" error={error} className="mt-8" />
      ) : tokens.length === 0 && !data.envToken ? (
        <div className="mt-8">
          <EmptyState
            icon={KeyIcon}
            title="No worker tokens"
            description="GPU workers can't connect."
          />
        </div>
      ) : (
        <Table className="mt-8 [--gutter:--spacing(6)] lg:[--gutter:--spacing(10)]">
          <TableHead>
            <TableRow>
              <TableHeader>Name</TableHeader>
              <TableHeader>Workers</TableHeader>
              <TableHeader>Created by</TableHeader>
              <TableHeader>Created</TableHeader>
              <TableHeader>Last used</TableHeader>
              <TableHeader className="relative w-0">
                <span className="sr-only">Actions</span>
              </TableHeader>
            </TableRow>
          </TableHead>
          <TableBody>
            {data.envToken && (
              <TableRow>
                <TableCell>
                  <span className="font-mono text-zinc-950 dark:text-white">
                    KRILL_WORKER_TOKEN
                  </span>{' '}
                </TableCell>
                <TableCell className="tabular-nums">
                  {plural(data.envTokenWorkers, 'worker')}
                </TableCell>
                <TableCell className="text-zinc-500 dark:text-zinc-400">—</TableCell>
                <TableCell className="text-zinc-500 dark:text-zinc-400">—</TableCell>
                <TableCell className="text-zinc-500 dark:text-zinc-400">—</TableCell>
                <TableCell />
              </TableRow>
            )}
            {tokens.map((t) => {
              const revoked = t.revokedAt !== undefined
              return (
                <TableRow key={String(t.id)} className={revoked ? 'opacity-60' : undefined}>
                  <TableCell>
                    <span className="font-medium text-zinc-950 dark:text-white">{t.name}</span>
                    {revoked && (
                      <Badge color="red" className="ml-2">
                        Revoked <When at={t.revokedAt} />
                      </Badge>
                    )}
                  </TableCell>
                  <TableCell className="tabular-nums">
                    {revoked ? '—' : plural(t.workers, 'worker')}
                  </TableCell>
                  <TableCell>
                    {t.createdBy ? (
                      <span className="flex items-center gap-2">
                        <Avatar seed={t.createdBy.id} className="size-6" />
                        {t.createdBy.name}
                      </span>
                    ) : (
                      <span className="text-zinc-500 dark:text-zinc-400">—</span>
                    )}
                  </TableCell>
                  <TableCell className="text-zinc-500 dark:text-zinc-400">
                    <When at={t.createdAt} />
                  </TableCell>
                  <TableCell className="text-zinc-500 dark:text-zinc-400">
                    <When at={t.lastUsedAt} />
                  </TableCell>
                  <TableCell>
                    {!revoked && (
                      <Button plain onClick={() => setRevoking(t)}>
                        Revoke
                      </Button>
                    )}
                  </TableCell>
                </TableRow>
              )
            })}
          </TableBody>
        </Table>
      )}

      <CreateDialog open={creating} onClose={() => setCreating(false)} />
      <ConfirmAlert
        open={revoking !== null}
        onClose={() => {
          setRevoking(null)
          revoke.reset()
        }}
        onConfirm={() => revoking && revoke.mutate({ id: revoking.id })}
        busy={revoke.isPending}
        error={revoke.error}
        title={`Revoke ${revoking?.name ?? ''}?`}
        description={
          revoking && revoking.workers > 0
            ? `${plural(revoking.workers, 'connected worker')} will be disconnected.`
            : 'Workers using it will be disconnected.'
        }
        confirmLabel="Revoke token"
      />
    </PageContentBlock>
  )
}
