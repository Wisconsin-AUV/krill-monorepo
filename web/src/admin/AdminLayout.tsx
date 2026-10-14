import { useQuery } from '@connectrpc/connect-query'
import { CloseButton } from '@headlessui/react'
import {
  ArrowLeftIcon,
  ChartBarIcon,
  ClockIcon,
  EnvelopeIcon,
  TableCellsIcon,
  ClipboardDocumentListIcon,
  ComputerDesktopIcon,
  CpuChipIcon,
  Cog6ToothIcon,
  HomeIcon,
  KeyIcon,
  QuestionMarkCircleIcon,
  TicketIcon,
  QueueListIcon,
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

const sections = [
  {
    heading: 'System',
    items: [
      { label: 'Overview', to: '/admin', icon: HomeIcon, permission: Permission.MANAGE_SYSTEM },
      {
        label: 'Settings',
        to: '/admin/settings',
        icon: Cog6ToothIcon,
        permission: Permission.MANAGE_SYSTEM,
      },
      {
        label: 'Emails',
        to: '/admin/emails',
        icon: EnvelopeIcon,
        permission: Permission.MANAGE_SYSTEM,
      },
    ],
  },
  {
    heading: 'Compute',
    items: [
      {
        label: 'Compute',
        to: '/admin/compute',
        icon: CpuChipIcon,
        permission: Permission.MANAGE_SYSTEM,
      },
      {
        label: 'Jobs',
        to: '/admin/jobs',
        icon: QueueListIcon,
        permission: Permission.MANAGE_SYSTEM,
      },
      {
        label: 'Worker tokens',
        to: '/admin/worker-tokens',
        icon: KeyIcon,
        permission: Permission.MANAGE_SYSTEM,
      },
    ],
  },
  {
    heading: 'People',
    items: [
      { label: 'Users', to: '/admin/users', icon: UsersIcon, permission: Permission.MANAGE_USERS },
      {
        label: 'Invites',
        to: '/admin/invites',
        icon: TicketIcon,
        permission: Permission.MANAGE_USERS,
      },
      {
        label: 'Sessions',
        to: '/admin/sessions',
        icon: ComputerDesktopIcon,
        permission: Permission.MANAGE_USERS,
      },
      {
        label: 'Audit log',
        to: '/admin/audit',
        icon: ClipboardDocumentListIcon,
        permission: Permission.MANAGE_SYSTEM,
      },
    ],
  },
  {
    heading: 'Labeling',
    items: [
      {
        label: 'Labelers',
        to: '/admin/labelers',
        icon: ChartBarIcon,
        permission: Permission.MANAGE_SYSTEM,
      },
      {
        label: 'Coverage',
        to: '/admin/coverage',
        icon: TableCellsIcon,
        permission: Permission.MANAGE_SYSTEM,
      },
      {
        label: 'Stuck clips',
        to: '/admin/stuck',
        icon: ClockIcon,
        permission: Permission.MANAGE_SYSTEM,
      },
    ],
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
            {sections.map(({ heading, items }) => {
              const visible = items.filter((i) => can(user, i.permission))
              if (visible.length === 0) return null
              return (
                <SidebarSection key={heading}>
                  <SidebarHeading>{heading}</SidebarHeading>
                  {visible.map(({ label, to, icon: Icon }) => (
                    <SidebarItem key={to} to={to} current={current(to)}>
                      <Icon data-slot="icon" />
                      <SidebarLabel>{label}</SidebarLabel>
                      {to === '/admin' && updateAvailable && <Dot label="Update available" />}
                    </SidebarItem>
                  ))}
                </SidebarSection>
              )
            })}
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
