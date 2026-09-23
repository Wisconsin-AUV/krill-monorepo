import { useQuery } from '@connectrpc/connect-query'
import { AuthService } from '@/gen/krill/v1/auth_pb'
import { Role, type User } from '@/gen/krill/v1/user_pb'
import { invalidateService } from './queryClient'

export function useSession() {
  return useQuery(AuthService.method.getSession, {}, { staleTime: Infinity, retry: false })
}

export function useUser(): User | undefined {
  return useSession().data?.user
}

export function hasRole(user: User | undefined, role: Role): boolean {
  return user !== undefined && user.role >= role
}

export function useHasRole(role: Role): boolean {
  return hasRole(useUser(), role)
}

export function refreshSession() {
  return invalidateService(AuthService)
}

export const roleOptions = [
  { value: Role.LABELER, label: 'Labeler', description: 'Labels clips.' },
  {
    value: Role.DEVELOPER,
    label: 'Developer',
    description: 'Also manages videos, label types, and exports.',
  },
  { value: Role.ADMIN, label: 'Admin', description: 'Also manages users.' },
] as const

export function roleLabel(role: Role): string {
  return roleOptions.find((o) => o.value === role)?.label ?? 'Unknown'
}

export function initials(name: string): string {
  const parts = name.trim().split(/\s+/).filter(Boolean)
  const letters = parts.length > 1 ? parts[0][0] + parts[parts.length - 1][0] : (parts[0] ?? '?')
  return letters.slice(0, 2).toUpperCase()
}

// Only same-origin paths, so ?next= can't send someone to another site.
export function safeNext(next: string | null): string {
  return next && /^\/(?![/\\])/.test(next) ? next : '/'
}

export function withNext(path: string, params: URLSearchParams): string {
  const next = params.get('next')
  return next ? `${path}?next=${encodeURIComponent(next)}` : path
}
