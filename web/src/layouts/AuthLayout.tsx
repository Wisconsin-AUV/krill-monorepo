import { Navigate, Outlet, useSearchParams } from 'react-router'
import { Credit } from '@/components/auth/Credit'
import { AuthLayout as Frame } from '@/components/ui/AuthLayout'
import LoadingSpinner from '@/components/ui/LoadingSpinner'
import { safeNext, useSession } from '@/lib/auth'

export function AuthLayout() {
  const { data, isPending } = useSession()
  const [params] = useSearchParams()

  if (data?.user) return <Navigate to={safeNext(params.get('next'))} replace />

  return (
    <Frame footer={data && <Credit team={data.teamName} />}>
      {isPending ? <LoadingSpinner /> : <Outlet />}
    </Frame>
  )
}
