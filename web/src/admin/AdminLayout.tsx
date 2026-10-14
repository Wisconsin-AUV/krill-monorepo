import { useQuery } from '@connectrpc/connect-query'
import { CloseButton } from '@headlessui/react'
import {
  ArrowLeftIcon,
  CpuChipIcon,
  Cog6ToothIcon,
  HomeIcon,
  QuestionMarkCircleIcon,
  UsersIcon,
} from '@heroicons/react/20/solid'
import { Navigate, Outlet, useLocation } from 'react-router'
import { Avatar } from '@/components/ui/Avatar'
import { Button } from '@/components/ui/Button'
import { Dropdown, DropdownButton } from '@/components/ui/Dropdown'
import { Navbar, NavbarItem, NavbarSection, NavbarSpacer } from '@/components/ui/Navbar'
import {
  Sidebar,
  SidebarBody,
  SidebarFooter,
  SidebarHeader,
  SidebarHeading,
  SidebarItem,
  SidebarLabel,
  SidebarSection,
} from '@/components/ui/Sidebar'
import { SidebarLayout } from '@/components/ui/SidebarLayout'
import { AdminService, UpdateState } from '@/gen/krill/v1/admin_pb'
import { Permission } from '@/gen/krill/v1/user_pb'
import { useChangelog } from '@/changelog/store'
import { UserAvatar, UserMenu } from '@/layouts/AppLayout'
import { can, canAdmin, useUser } from '@/lib/auth'

const items = [
  { label: 'Overview', to: '/admin', icon: HomeIcon, permission: Permission.MANAGE_SYSTEM },
  { label: 'Users', to: '/admin/users', icon: UsersIcon, permission: Permission.MANAGE_USERS },
  {
    label: 'Settings',
    to: '/admin/settings',
    icon: Cog6ToothIcon,
    permission: Permission.MANAGE_SYSTEM,
  },
  {
    label: 'Compute',
    to: '/admin/compute',
    icon: CpuChipIcon,
    permission: Permission.MANAGE_SYSTEM,
  },
]

function Dot({ label }: { label: string }) {
  return (
    <span className="ml-auto size-2 rounded-full bg-sky-500">
      <span className="sr-only">{label}</span>
    </span>
  )
}

export function AdminLayout() {
  const user = useUser()
  const { pathname } = useLocation()
  const system = can(user, Permission.MANAGE_SYSTEM)
  const { data: status } = useQuery(AdminService.method.getStatus, {}, { enabled: system })
  const showChangelog = useChangelog((s) => s.show)

  if (!canAdmin(user)) return <Navigate to="/" replace />
  if (pathname === '/admin' && !system) return <Navigate to="/admin/users" replace />

  const updateAvailable = status?.version?.state === UpdateState.AVAILABLE
  const current = (to: string) =>
    to === '/admin' ? pathname === '/admin' : pathname.startsWith(to)

  return (
    <SidebarLayout
      navbar={
        <Navbar>
          <NavbarSpacer />
          <NavbarSection>
            {user && (
              <Dropdown>
                <DropdownButton as={NavbarItem} aria-label="Account menu">
                  <UserAvatar user={user} />
                </DropdownButton>
                <UserMenu user={user} anchor="bottom end" />
              </Dropdown>
            )}
          </NavbarSection>
        </Navbar>
      }
      sidebar={
        <Sidebar>
          <SidebarHeader>
            <div className="flex items-center gap-2">
              <SidebarItem to="/" className="flex-1">
                <ArrowLeftIcon data-slot="icon" />
                <SidebarLabel>Back to Krill</SidebarLabel>
              </SidebarItem>
              {/* The account menu with the changelog is hidden on phones. */}
              <CloseButton
                as={Button}
                plain
                onClick={() => showChangelog()}
                aria-label="Changelog"
                className="lg:hidden"
              >
                <QuestionMarkCircleIcon data-slot="icon" />
              </CloseButton>
            </div>
          </SidebarHeader>
          <SidebarBody>
            <SidebarSection>
              <SidebarHeading>Admin</SidebarHeading>
              {items
                .filter((i) => can(user, i.permission))
                .map(({ label, to, icon: Icon }) => (
                  <SidebarItem key={to} to={to} current={current(to)}>
                    <Icon data-slot="icon" />
                    <SidebarLabel>{label}</SidebarLabel>
                    {to === '/admin' && updateAvailable && <Dot label="Update available" />}
                  </SidebarItem>
                ))}
            </SidebarSection>
          </SidebarBody>
          {user && (
            <SidebarFooter className="max-lg:hidden">
              <Dropdown>
                <DropdownButton as={SidebarItem}>
                  <span className="flex min-w-0 items-center gap-3">
                    <Avatar seed={user.id} className="size-8 shrink-0" />
                    <span className="min-w-0">
                      <span className="block truncate text-sm/5 font-medium text-zinc-950 dark:text-white">
                        {user.name}
                      </span>
                      <span className="block truncate text-xs/5 font-normal text-zinc-500 dark:text-zinc-400">
                        {user.email}
                      </span>
                    </span>
                  </span>
                </DropdownButton>
                <UserMenu user={user} anchor="top start" changelog />
              </Dropdown>
            </SidebarFooter>
          )}
        </Sidebar>
      }
    >
      <Outlet />
    </SidebarLayout>
  )
}
