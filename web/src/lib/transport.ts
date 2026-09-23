import { Code, ConnectError, type Interceptor } from '@connectrpc/connect'
import { createConnectTransport } from '@connectrpc/connect-web'
import { AuthService } from '@/gen/krill/v1/auth_pb'
import { invalidateService } from './queryClient'

// A session can expire or be revoked mid-use. Refetching it sends the user
// back to the login page.
const refetchSessionOnUnauthenticated: Interceptor = (next) => async (req) => {
  try {
    return await next(req)
  } catch (err) {
    if (
      err instanceof ConnectError &&
      err.code === Code.Unauthenticated &&
      req.service.typeName !== AuthService.typeName
    ) {
      void invalidateService(AuthService)
    }
    throw err
  }
}

export const transport = createConnectTransport({
  baseUrl: window.location.origin,
  interceptors: [refetchSessionOnUnauthenticated],
})
