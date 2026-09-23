import { Navigate, Outlet, useSearchParams } from 'react-router'
import LoadingSpinner from '@/components/ui/LoadingSpinner'
import { safeNext, useSession } from '@/lib/auth'

export function AuthLayout() {
  const { data, isPending } = useSession()
  const [params] = useSearchParams()

  if (data?.user) return <Navigate to={safeNext(params.get('next'))} replace />

  return (
    <main className="flex min-h-svh flex-col items-center justify-center bg-zinc-100 p-6 dark:bg-zinc-950">
      <div className="w-full max-w-sm rounded-2xl bg-white p-8 shadow-xs ring-1 ring-zinc-950/5 dark:bg-zinc-900 dark:ring-white/10">
        <div className="text-lg/7 font-semibold text-zinc-950 dark:text-white">Krill</div>
        {isPending ? (
          <div className="flex justify-center py-16">
            <LoadingSpinner />
          </div>
        ) : (
          <Outlet />
        )}
      </div>
    </main>
  )
}
