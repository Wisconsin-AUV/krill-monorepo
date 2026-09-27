import { TransportProvider } from '@connectrpc/connect-query'
import type { Transport } from '@connectrpc/connect'
import { QueryClientProvider } from '@tanstack/react-query'
import { RouterProvider } from 'react-router'
import { queryClient } from '@/lib/queryClient'
import { transport as defaultTransport } from '@/lib/transport'
import { router } from '@/router'

export default function App({ transport = defaultTransport }: { transport?: Transport }) {
  return (
    <TransportProvider transport={transport}>
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>
    </TransportProvider>
  )
}
