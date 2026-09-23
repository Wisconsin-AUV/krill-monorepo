import { useMutation, useQuery } from '@connectrpc/connect-query'
import {
  ArrowDownTrayIcon,
  ArrowRightStartOnRectangleIcon,
  BookOpenIcon,
  FilmIcon,
  TagIcon,
  UserCircleIcon,
  UsersIcon,
} from '@heroicons/react/20/solid'
import { Outlet, useLocation, useNavigate } from 'react-router'
import { Avatar } from '@/components/ui/Avatar'
import {
  Dropdown,
  DropdownButton,
  DropdownDivider,
  DropdownHeader,
  DropdownItem,
  DropdownLabel,
  DropdownMenu,
} from '@/components/ui/Dropdown'
import FlashMessageRender from '@/components/ui/FlashMessageRender'
import {
  Navbar,
  NavbarDivider,
  NavbarItem,
  NavbarLabel,
  NavbarSection,
  NavbarSpacer,
} from '@/components/ui/Navbar'
import ProgressBar from '@/components/ui/ProgressBar'
import {
  Sidebar,
  SidebarBody,
  SidebarFooter,
  SidebarHeader,
  SidebarItem,
  SidebarLabel,
  SidebarSection,
} from '@/components/ui/Sidebar'
import { StackedLayout } from '@/components/ui/StackedLayout'
import { UploadTray } from '@/components/UploadTray'
import { AuthService } from '@/gen/krill/v1/auth_pb'
import { HealthService } from '@/gen/krill/v1/health_pb'
import { Role, type User } from '@/gen/krill/v1/user_pb'
import { hasRole, initials, roleLabel, useUser } from '@/lib/auth'
import { flash } from '@/lib/flash'
import { GUIDELINE_URL } from '@/lib/links'
import { queryClient } from '@/lib/queryClient'

const navItems = [
  {
    label: 'Videos',
    to: '/',
    icon: FilmIcon,
    role: Role.LABELER,
    match: (p: string) => p === '/' || p.startsWith('/videos'),
  },
  {
    label: 'Labels',
    to: '/labels',
    icon: TagIcon,
    role: Role.DEVELOPER,
    match: (p: string) => p.startsWith('/labels'),
  },
  {
    label: 'Exports',
    to: '/exports',
    icon: ArrowDownTrayIcon,
    role: Role.DEVELOPER,
    match: (p: string) => p.startsWith('/exports'),
  },
  {
    label: 'Users',
    to: '/users',
    icon: UsersIcon,
    role: Role.ADMIN,
    match: (p: string) => p.startsWith('/users'),
  },
]

function UserAvatar({ user }: { user: User }) {
  return <Avatar initials={initials(user.name)} className="rounded-full bg-sky-600" />
}

function UserMenu({ user, anchor }: { user: User; anchor: 'bottom end' | 'top start' }) {
  const navigate = useNavigate()
  const logout = useMutation(AuthService.method.logout, {
    onSuccess: () => {
      queryClient.clear()
      navigate('/login')
    },
    onError: (err) => flash.error('Could not sign out', err),
  })

  return (
    <DropdownMenu className="min-w-56" anchor={anchor}>
      <DropdownHeader>
        <div className="text-sm/5 font-medium text-zinc-950 dark:text-white">{user.name}</div>
        <div className="text-xs/5 text-zinc-500 dark:text-zinc-400">
          @{user.username} · {roleLabel(user.role)}
        </div>
      </DropdownHeader>
      <DropdownDivider />
      <DropdownItem to="/account">
        <UserCircleIcon data-slot="icon" />
        <DropdownLabel>Account</DropdownLabel>
      </DropdownItem>
      <DropdownItem onClick={() => logout.mutate({})} disabled={logout.isPending}>
        <ArrowRightStartOnRectangleIcon data-slot="icon" />
        <DropdownLabel>Sign out</DropdownLabel>
      </DropdownItem>
    </DropdownMenu>
  )
}

function Version() {
  const { data } = useQuery(HealthService.method.check, {})
  if (!data) return null
  return (
    <span className="text-xs text-zinc-500 tabular-nums dark:text-zinc-400">{data.version}</span>
  )
}

// Toasts, the progress bar, and uploads live above every layout so they
// survive moving between the app and the tagger.
export function AppChrome() {
  return (
    <>
      <ProgressBar />
      <FlashMessageRender byKey="global" />
      <Outlet />
      <UploadTray />
    </>
  )
}

export function AppLayout() {
  const { pathname } = useLocation()
  const user = useUser()
  const items = navItems.filter((i) => hasRole(user, i.role))

  return (
    <>
      <StackedLayout
        navbar={
          <Navbar>
            <NavbarItem to="/" current={false} className="max-lg:hidden">
              <NavbarLabel className="font-semibold">Krill</NavbarLabel>
            </NavbarItem>
            <NavbarDivider className="max-lg:hidden" />
            <NavbarSection className="max-lg:hidden">
              {items.map(({ label, to, icon: Icon, match }) => (
                <NavbarItem key={to} to={to} current={match(pathname)}>
                  <Icon data-slot="icon" />
                  <NavbarLabel>{label}</NavbarLabel>
                </NavbarItem>
              ))}
            </NavbarSection>
            <NavbarSpacer />
            <NavbarSection>
              <NavbarItem to={GUIDELINE_URL} target="_blank" rel="noreferrer">
                <BookOpenIcon data-slot="icon" />
                <NavbarLabel className="max-sm:hidden">Guideline</NavbarLabel>
              </NavbarItem>
              <Version />
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
              <SidebarItem to="/">
                <SidebarLabel className="font-semibold">Krill</SidebarLabel>
              </SidebarItem>
            </SidebarHeader>
            <SidebarBody>
              <SidebarSection>
                {items.map(({ label, to, icon: Icon, match }) => (
                  <SidebarItem key={to} to={to} current={match(pathname)}>
                    <Icon data-slot="icon" />
                    <SidebarLabel>{label}</SidebarLabel>
                  </SidebarItem>
                ))}
              </SidebarSection>
            </SidebarBody>
            <SidebarFooter>
              <SidebarItem to={GUIDELINE_URL} target="_blank" rel="noreferrer">
                <BookOpenIcon data-slot="icon" />
                <SidebarLabel>Labeling guideline</SidebarLabel>
              </SidebarItem>
              {user && (
                <Dropdown>
                  <DropdownButton as={SidebarItem}>
                    <UserAvatar user={user} />
                    <SidebarLabel>{user.name}</SidebarLabel>
                  </DropdownButton>
                  <UserMenu user={user} anchor="top start" />
                </Dropdown>
              )}
            </SidebarFooter>
          </Sidebar>
        }
      >
        <Outlet />
      </StackedLayout>
    </>
  )
}
