import { useMutation } from '@connectrpc/connect-query'
import {
  ArrowRightStartOnRectangleIcon,
  ArrowsPointingInIcon,
  CheckBadgeIcon,
  EllipsisHorizontalIcon,
  EnvelopeIcon,
  KeyIcon,
  NoSymbolIcon,
  ArrowUturnLeftIcon,
  TrashIcon,
} from '@heroicons/react/20/solid'
import { useState } from 'react'
import { ConfirmAlert } from '@/components/ConfirmAlert'
import { PasswordInput } from '@/components/PasswordInput'
import { Button } from '@/components/ui/Button'
import { FormError } from '@/components/ui/Callout'
import { Dialog, DialogActions, DialogBody, DialogTitle } from '@/components/ui/Dialog'
import {
  Dropdown,
  DropdownButton,
  DropdownDivider,
  DropdownItem,
  DropdownLabel,
  DropdownMenu,
} from '@/components/ui/Dropdown'
import { Description, Field, Label } from '@/components/ui/Fieldset'
import { SessionService } from '@/gen/krill/v1/session_pb'
import { UserService, type User } from '@/gen/krill/v1/user_pb'
import { useSession } from '@/lib/auth'
import { flash } from '@/lib/flash'
import { plural } from '@/lib/format'
import { usePasswordCheck } from '@/lib/password'
import { invalidateService } from '@/lib/queryClient'
import { MergeDialog } from './MergeDialog'

function PasswordDialog({ user, onClose }: { user: User | null; onClose: () => void }) {
  const [password, setPassword] = useState('')
  const check = usePasswordCheck(password, user ? [user.name, user.username, user.email] : [])
  const set = useMutation(UserService.method.setUserPassword, {
    onSuccess: async () => {
      await invalidateService(SessionService)
      flash.success('Password set', `${user?.name} was signed out everywhere.`)
      setPassword('')
      onClose()
    },
  })
  const close = () => {
    set.reset()
    onClose()
  }

  return (
    <Dialog open={user !== null} onClose={close} size="md">
      <form
        onSubmit={(e) => {
          e.preventDefault()
          if (user) set.mutate({ id: user.id, password })
        }}
      >
        <input
          type="text"
          name="username"
          autoComplete="username"
          value={user?.username ?? ''}
          readOnly
          hidden
        />
        <DialogTitle>Set password for {user?.name}</DialogTitle>
        <DialogBody>
          <Field>
            <Label>New password</Label>
            <Description>Share it with them directly. They can change it later.</Description>
            <PasswordInput value={password} onChange={setPassword} check={check} />
          </Field>
          <FormError error={set.error} className="mt-6" />
        </DialogBody>
        <DialogActions>
          <Button plain onClick={close}>
            Cancel
          </Button>
          <Button type="submit" color="sky" disabled={set.isPending || !check.ok}>
            Set password
          </Button>
        </DialogActions>
      </form>
    </Dialog>
  )
}

function refresh() {
  return Promise.all([invalidateService(UserService), invalidateService(SessionService)])
}

// UserActions is the menu of things an admin can do to another account.
export function UserActions({
  user,
  users,
  onDeleted,
}: {
  user: User
  users: User[]
  onDeleted?: () => void
}) {
  // Unverified emails only matter when sign-up checks them with a code.
  const verifying = useSession().data?.emailVerificationRequired ?? false
  const [dialog, setDialog] = useState<'password' | 'merge' | 'deactivate' | 'delete' | null>(null)
  const close = () => setDialog(null)

  const signOut = useMutation(UserService.method.signOutUser, {
    onSuccess: async () => {
      await refresh()
      flash.success('Signed out', `${user.name} will need to sign in again.`)
    },
    onError: (err) => flash.error('Could not sign out user', err),
  })
  const verify = useMutation(UserService.method.markEmailVerified, {
    onSuccess: () => invalidateService(UserService),
    onError: (err) => flash.error('Could not verify email', err),
  })
  const reset = useMutation(UserService.method.sendPasswordReset, {
    onSuccess: () => flash.success('Reset code sent', user.email),
    onError: (err) => flash.error('Could not send a reset code', err),
  })
  const deactivate = useMutation(UserService.method.deactivateUser, {
    onSuccess: async (res) => {
      await refresh()
      close()
      flash.success(
        `${user.name} deactivated`,
        `${plural(res.sessionsEnded, 'session')} ended · ${plural(res.claimsReleased, 'clip')} released`,
      )
    },
  })
  const reactivate = useMutation(UserService.method.reactivateUser, {
    onSuccess: async () => {
      await refresh()
      flash.success(`${user.name} reactivated`)
    },
    onError: (err) => flash.error('Could not reactivate user', err),
  })
  const remove = useMutation(UserService.method.deleteUser, {
    onSuccess: async () => {
      await refresh()
      close()
      onDeleted?.()
    },
  })

  return (
    <>
      <Dropdown>
        <DropdownButton plain aria-label={`Options for ${user.name}`}>
          <EllipsisHorizontalIcon data-slot="icon" />
        </DropdownButton>
        <DropdownMenu anchor="bottom end">
          <DropdownItem onClick={() => setDialog('password')}>
            <KeyIcon data-slot="icon" />
            <DropdownLabel>Set password</DropdownLabel>
          </DropdownItem>
          <DropdownItem
            onClick={() => reset.mutate({ id: user.id })}
            disabled={user.disabled || reset.isPending}
          >
            <EnvelopeIcon data-slot="icon" />
            <DropdownLabel>Email a reset code</DropdownLabel>
          </DropdownItem>
          {verifying && !user.emailVerified && (
            <DropdownItem onClick={() => verify.mutate({ id: user.id })}>
              <CheckBadgeIcon data-slot="icon" />
              <DropdownLabel>Mark email verified</DropdownLabel>
            </DropdownItem>
          )}
          <DropdownItem
            onClick={() => signOut.mutate({ id: user.id })}
            disabled={user.activeSessions === 0}
          >
            <ArrowRightStartOnRectangleIcon data-slot="icon" />
            <DropdownLabel>
              {user.activeSessions > 0
                ? `Sign out of ${plural(user.activeSessions, 'session')}`
                : 'Not signed in'}
            </DropdownLabel>
          </DropdownItem>
          <DropdownDivider />
          <DropdownItem onClick={() => setDialog('merge')}>
            <ArrowsPointingInIcon data-slot="icon" />
            <DropdownLabel>Merge…</DropdownLabel>
          </DropdownItem>
          {user.disabled ? (
            <DropdownItem onClick={() => reactivate.mutate({ id: user.id })}>
              <ArrowUturnLeftIcon data-slot="icon" />
              <DropdownLabel>Reactivate</DropdownLabel>
            </DropdownItem>
          ) : (
            <DropdownItem onClick={() => setDialog('deactivate')}>
              <NoSymbolIcon data-slot="icon" />
              <DropdownLabel>Deactivate</DropdownLabel>
            </DropdownItem>
          )}
          <DropdownItem onClick={() => setDialog('delete')}>
            <TrashIcon data-slot="icon" />
            <DropdownLabel>Delete</DropdownLabel>
          </DropdownItem>
        </DropdownMenu>
      </Dropdown>

      <PasswordDialog user={dialog === 'password' ? user : null} onClose={close} />
      <MergeDialog user={dialog === 'merge' ? user : null} users={users} onClose={close} />
      <ConfirmAlert
        open={dialog === 'deactivate'}
        onClose={() => {
          close()
          deactivate.reset()
        }}
        onConfirm={() => deactivate.mutate({ id: user.id })}
        busy={deactivate.isPending}
        error={deactivate.error}
        title={`Deactivate ${user.name}?`}
        description="Signs them out everywhere and releases their clips. Their labels stay."
        confirmLabel="Deactivate"
      />
      <ConfirmAlert
        open={dialog === 'delete'}
        onClose={() => {
          close()
          remove.reset()
        }}
        onConfirm={() => remove.mutate({ id: user.id })}
        busy={remove.isPending}
        error={remove.error}
        title={`Delete ${user.name}?`}
        description="Their labels stay. Deactivate the account instead if they might come back."
        confirmLabel="Delete user"
      />
    </>
  )
}
