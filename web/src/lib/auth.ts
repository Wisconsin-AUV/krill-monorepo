import { useQuery } from '@connectrpc/connect-query'
import { AuthService } from '@/gen/krill/v1/auth_pb'
import { Role, type Permission, type User } from '@/gen/krill/v1/user_pb'
import { invalidateService } from './queryClient'

export function useSession() {
  return useQuery(AuthService.method.getSession, {}, { staleTime: Infinity, retry: false })
}

export function useUser(): User | undefined {
  return useSession().data?.user
}

export function can(user: User | undefined, permission: Permission): boolean {
  return user?.permissions.includes(permission) ?? false
}

export function useCan(permission: Permission): boolean {
  return can(useUser(), permission)
}

// Which role grants a permission, from the API's single list.
export function usePermissionRole(permission: Permission): Role | undefined {
  return useSession().data?.allPermissions.find((p) => p.permission === permission)?.role
}

export function refreshSession() {
  return invalidateService(AuthService)
}

// Roles and their names come from the API, lowest first.
export function useRoles() {
  return useSession().data?.roles ?? []
}

export function useRoleLabel(): (role: Role) => string {
  const roles = useRoles()
  return (role) => roles.find((r) => r.role === role)?.label ?? ''
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

export type LoginMethod = 'password' | 'slack'

// Set by the API after a successful sign-in.
export function lastLoginMethod(cookies = document.cookie): LoginMethod | null {
  const value = cookies
    .split('; ')
    .find((c) => c.startsWith('krill_last_login='))
    ?.split('=')[1]
  return value === 'password' || value === 'slack' ? value : null
}
