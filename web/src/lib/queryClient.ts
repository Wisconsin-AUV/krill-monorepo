import { createConnectQueryKey } from '@connectrpc/connect-query'
import type { DescService } from '@bufbuild/protobuf'
import { QueryClient } from '@tanstack/react-query'

export const queryClient = new QueryClient({
  defaultOptions: { queries: { refetchOnWindowFocus: false, retry: 1 } },
})

export function invalidateService(schema: DescService) {
  return queryClient.invalidateQueries({
    queryKey: createConnectQueryKey({ schema, cardinality: undefined }),
  })
}
