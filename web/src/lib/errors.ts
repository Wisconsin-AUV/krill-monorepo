import { Code, ConnectError } from '@connectrpc/connect'

export function errorMessage(err: unknown): string {
  if (err instanceof ConnectError) {
    if (err.code === Code.Unavailable) return 'Could not reach the server'
    if (err.code === Code.Internal || err.code === Code.Unknown) return 'Something went wrong'
    // Server messages follow Go style and start lowercase.
    return err.rawMessage.charAt(0).toUpperCase() + err.rawMessage.slice(1)
  }
  if (err instanceof Error) return err.message
  return String(err)
}
