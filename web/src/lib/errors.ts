import { ConnectError } from '@connectrpc/connect'

export function errorMessage(err: unknown): string {
  if (err instanceof ConnectError) return err.rawMessage
  if (err instanceof Error) return err.message
  return String(err)
}
