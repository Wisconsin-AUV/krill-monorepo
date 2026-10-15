import { timestampDate } from '@bufbuild/protobuf/wkt'
import { useMutation } from '@connectrpc/connect-query'
import { SessionService, type Session } from '@/gen/krill/v1/session_pb'
import { UserService } from '@/gen/krill/v1/user_pb'
import { flash } from './flash'
import { invalidateService } from './queryClient'

// The API marks a session seen at most every five minutes.
const ACTIVE_MS = 10 * 60 * 1000

export function isActive(s: Session, now = Date.now()) {
  return s.lastSeenAt !== undefined && now - timestampDate(s.lastSeenAt).getTime() < ACTIVE_MS
}

// useRevokeSession revokes anyone's session, for admins.
export function useRevokeSession() {
  return useMutation(SessionService.method.revokeSession, {
    onSuccess: async () => {
      await Promise.all([invalidateService(SessionService), invalidateService(UserService)])
      flash.success('Session revoked')
    },
    onError: (err) => flash.error('Could not revoke session', err),
  })
}
