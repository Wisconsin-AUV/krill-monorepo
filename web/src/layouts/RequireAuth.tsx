import { Navigate, Outlet, useLocation } from 'react-router'
import { Button } from '@/components/ui/Button'
import { Heading } from '@/components/ui/Heading'
import LoadingSpinner from '@/components/ui/LoadingSpinner'
import PageContentBlock from '@/components/ui/PageContentBlock'
import { Text } from '@/components/ui/Text'
import type { Permission } from '@/gen/krill/v1/user_pb'
import { refreshSession, useCan, usePermissionRole, useRoleLabel, useSession } from '@/lib/auth'
import { errorMessage } from '@/lib/errors'

export function RequireAuth() {
  const { data, isPending, error } = useSession()
  const { pathname, search } = useLocation()

  if (isPending) {
    return (
      <div className="flex h-svh items-center justify-center">
        <LoadingSpinner />
      </div>
    )
  }
  if (error) {
    return (
      <div className="flex h-svh flex-col items-center justify-center gap-4 p-6 text-center">
        <Text>Could not reach Krill: {errorMessage(error)}</Text>
        <Button outline onClick={() => refreshSession()}>
          Try again
        </Button>
      </div>
    )
  }
  if (!data.user) {
    const next = pathname === '/' ? '' : `?next=${encodeURIComponent(pathname + search)}`
    return <Navigate to={`/login${next}`} replace />
  }
  return <Outlet />
}

export function RequirePermission({ permission }: { permission: Permission }) {
  const allowed = useCan(permission)
  const role = usePermissionRole(permission)
  const roleLabel = useRoleLabel()
  if (allowed) return <Outlet />
  return (
    <PageContentBlock title="No access · Krill">
      <div className="py-24 text-center">
        <p className="text-sm font-semibold text-sky-600 dark:text-sky-400">403</p>
        <Heading className="mt-2">You don't have access to this page</Heading>
        <Text className="mt-2">
          {role ? `It needs the ${roleLabel(role).toLowerCase()} role. ` : ''}Ask an admin if you
          need it.
        </Text>
        <Button to="/" className="mt-8">
          Back home
        </Button>
      </div>
    </PageContentBlock>
  )
}
