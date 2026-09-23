import { useQuery } from '@connectrpc/connect-query'
import { BookOpenIcon, FilmIcon, TagIcon } from '@heroicons/react/20/solid'
import { Link, Outlet, useLocation } from 'react-router'
import { Logo } from '@/components/Logo'
import FlashMessageRender from '@/components/ui/FlashMessageRender'
import {
  Navbar,
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
import { HealthService } from '@/gen/krill/v1/health_pb'
import { GUIDELINE_URL } from '@/lib/links'

const navItems = [
  {
    label: 'Videos',
    to: '/',
    icon: FilmIcon,
    match: (p: string) => p === '/' || p.startsWith('/videos'),
  },
  { label: 'Labels', to: '/labels', icon: TagIcon, match: (p: string) => p.startsWith('/labels') },
]

function Version() {
  const { data } = useQuery(HealthService.method.check, {})
  if (!data) return null
  return (
    <span className="text-xs text-zinc-500 tabular-nums dark:text-zinc-400">{data.version}</span>
  )
}

export function AppLayout() {
  const { pathname } = useLocation()

  return (
    <>
      <ProgressBar />
      <FlashMessageRender byKey="global" />
      <StackedLayout
        navbar={
          <Navbar>
            <Link to="/" aria-label="Home" className="rounded-lg px-2 py-2.5 max-lg:hidden">
              <Logo />
            </Link>
            <NavbarSection className="max-lg:hidden">
              {navItems.map(({ label, to, icon: Icon, match }) => (
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
            </NavbarSection>
          </Navbar>
        }
        sidebar={
          <Sidebar>
            <SidebarHeader>
              <Logo className="px-2" />
            </SidebarHeader>
            <SidebarBody>
              <SidebarSection>
                {navItems.map(({ label, to, icon: Icon, match }) => (
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
            </SidebarFooter>
          </Sidebar>
        }
      >
        <Outlet />
      </StackedLayout>
      <UploadTray />
    </>
  )
}
