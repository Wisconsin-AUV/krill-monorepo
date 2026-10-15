import { useMutation, useQuery } from '@connectrpc/connect-query'
import {
  ArrowDownTrayIcon,
  ArrowRightStartOnRectangleIcon,
  BookOpenIcon,
  ChartBarIcon,
  FilmIcon,
  FlagIcon,
  HomeIcon,
  Squares2X2Icon,
  TagIcon,
  ShieldCheckIcon,
  SparklesIcon,
  TrophyIcon,
  UserCircleIcon,
} from '@heroicons/react/20/solid'
import { lazy, Suspense, useState } from 'react'
import { Outlet, useLocation, useNavigate } from 'react-router'
import { Avatar } from '@/components/ui/Avatar'
import { Badge } from '@/components/ui/Badge'
import {
  Dropdown,
  DropdownButton,
  DropdownDivider,
  DropdownHeader,
  DropdownItem,
  DropdownLabel,
  DropdownMenu,
} from '@/components/ui/Dropdown'
import { FlashMessageRender } from '@/components/ui/FlashMessageRender'
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
import { HelpMenu } from '@/changelog/HelpMenu'
import { useChangelog } from '@/changelog/store'
import { AuthService } from '@/gen/krill/v1/auth_pb'
import { ReviewService } from '@/gen/krill/v1/review_pb'
import { Permission, type User } from '@/gen/krill/v1/user_pb'
import { can, canAdmin, useRoleLabel, useUser } from '@/lib/auth'
import { flash } from '@/lib/flash'
import { GUIDELINE_URL } from '@/lib/links'
import { queryClient } from '@/lib/queryClient'
import { profilePath } from '@/lib/stats'

const navItems = [
  {
    label: 'Home',
    to: '/',
    icon: HomeIcon,
    permission: Permission.LABEL,
    match: (p: string) => p === '/',
  },
  {
    label: 'Videos',
    to: '/videos',
    icon: FilmIcon,
    permission: Permission.MANAGE_VIDEOS,
    match: (p: string) => p.startsWith('/videos'),
  },
  {
    label: 'Attributes',
    to: '/attributes',
    icon: Squares2X2Icon,
    permission: Permission.LABEL,
    match: (p: string) => p.startsWith('/attributes'),
  },
  {
    label: 'Leaderboard',
    to: '/leaderboard',
    icon: TrophyIcon,
    permission: Permission.LABEL,
    match: (p: string) => p.startsWith('/leaderboard'),
  },
  {
    label: 'Labels',
    to: '/labels',
    icon: TagIcon,
    permission: Permission.MANAGE_LABEL_TYPES,
    match: (p: string) => p.startsWith('/labels'),
  },
  {
    label: 'Exports',
    to: '/exports',
    icon: ArrowDownTrayIcon,
    permission: Permission.MANAGE_EXPORTS,
    match: (p: string) => p.startsWith('/exports'),
  },
  {
    label: 'Review',
    to: '/review',
    icon: FlagIcon,
    permission: Permission.REVIEW_LABELS,
    match: (p: string) => p.startsWith('/review'),
  },
]

export function UserAvatar({ user }: { user: User }) {
  return <Avatar seed={user.id} />
}

export function UserMenu({
  user,
  anchor,
  changelog = false,
}: {
  user: User
  anchor: 'bottom end' | 'top start'
  // For layouts without the help menu in their top bar.
  changelog?: boolean
}) {
  const roleLabel = useRoleLabel()
  const showChangelog = useChangelog((s) => s.show)
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
      <DropdownItem to={profilePath(user.username)}>
        <ChartBarIcon data-slot="icon" />
        <DropdownLabel>Profile</DropdownLabel>
      </DropdownItem>
      <DropdownItem to="/account">
        <UserCircleIcon data-slot="icon" />
        <DropdownLabel>Account</DropdownLabel>
      </DropdownItem>
      {canAdmin(user) && (
        <DropdownItem to="/admin">
          <ShieldCheckIcon data-slot="icon" />
          <DropdownLabel>Admin</DropdownLabel>
        </DropdownItem>
      )}
      {changelog && (
        <DropdownItem onClick={() => showChangelog()}>
          <SparklesIcon data-slot="icon" />
          <DropdownLabel>Changelog</DropdownLabel>
        </DropdownItem>
      )}
      <DropdownItem onClick={() => logout.mutate({})} disabled={logout.isPending}>
        <ArrowRightStartOnRectangleIcon data-slot="icon" />
        <DropdownLabel>Sign out</DropdownLabel>
      </DropdownItem>
    </DropdownMenu>
  )
}

const ChangelogDialog = lazy(() =>
  import('@/changelog/ChangelogDialog').then((m) => ({ default: m.ChangelogDialog })),
)

// Mounted on first open and kept, so closing still animates.
function Changelog() {
  const open = useChangelog((s) => s.open)
  const [loaded, setLoaded] = useState(false)
  if (open && !loaded) setLoaded(true)
  if (!loaded) return null
  return (
    <Suspense>
      <ChangelogDialog />
    </Suspense>
  )
}

// Toasts, the progress bar, and uploads live above every layout so they
// survive moving between the app and the tagger.
export function AppChrome() {
  return (
    <>
      <ProgressBar />
      <FlashMessageRender />
      <Outlet />
      <UploadTray />
      <Changelog />
    </>
  )
}

export function AppLayout() {
  const { pathname } = useLocation()
  const user = useUser()
  const items = navItems.filter((i) => can(user, i.permission))
  const { data: reviews } = useQuery(
    ReviewService.method.countReviews,
    {},
    { enabled: can(user, Permission.REVIEW_LABELS) },
  )
  const counts: Record<string, number> = { '/review': reviews?.count ?? 0 }

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
                  {counts[to] > 0 && <Badge className="-my-0.5">{counts[to]}</Badge>}
                </NavbarItem>
              ))}
            </NavbarSection>
            <NavbarSpacer />
            <NavbarSection>
              <HelpMenu />
              {canAdmin(user) && (
                <NavbarItem
                  to="/admin"
                  current={false}
                  aria-label="Admin"
                  className="max-lg:hidden"
                >
                  <ShieldCheckIcon data-slot="icon" />
                </NavbarItem>
              )}
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
                    {counts[to] > 0 && <Badge className="ml-auto">{counts[to]}</Badge>}
                  </SidebarItem>
                ))}
              </SidebarSection>
            </SidebarBody>
            <SidebarFooter>
              {canAdmin(user) && (
                <SidebarItem to="/admin">
                  <ShieldCheckIcon data-slot="icon" />
                  <SidebarLabel>Admin</SidebarLabel>
                </SidebarItem>
              )}
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
