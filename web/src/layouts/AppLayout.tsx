import { useQuery } from '@connectrpc/connect-query'
import { ArrowDownTrayIcon, BookOpenIcon, FilmIcon, TagIcon } from '@heroicons/react/20/solid'
import { Outlet, useLocation } from 'react-router'
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
  {
    label: 'Exports',
    to: '/exports',
    icon: ArrowDownTrayIcon,
    match: (p: string) => p.startsWith('/exports'),
  },
]

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
              <SidebarItem to="/">
                <SidebarLabel className="font-semibold">Krill</SidebarLabel>
              </SidebarItem>
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
    </>
  )
}
