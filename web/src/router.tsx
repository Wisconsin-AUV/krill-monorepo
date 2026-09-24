import { createBrowserRouter } from 'react-router'
import LoadingSpinner from '@/components/ui/LoadingSpinner'
import { Permission } from '@/gen/krill/v1/user_pb'
import { AppChrome, AppLayout } from '@/layouts/AppLayout'
import { AuthLayout } from '@/layouts/AuthLayout'
import { RequireAuth, RequirePermission } from '@/layouts/RequireAuth'
import { AccountPage } from '@/pages/AccountPage'
import { ExportsPage } from '@/pages/ExportsPage'
import { LabelsPage } from '@/pages/LabelsPage'
import { LeaderboardPage } from '@/pages/LeaderboardPage'
import { LoginPage } from '@/pages/LoginPage'
import { NotFound } from '@/pages/NotFound'
import { RegisterPage } from '@/pages/RegisterPage'
import { UsersPage } from '@/pages/UsersPage'
import { VideoPage } from '@/pages/VideoPage'
import { VideosPage } from '@/pages/VideosPage'

export const router = createBrowserRouter([
  {
    element: <AppChrome />,
    hydrateFallbackElement: (
      <div className="flex h-svh items-center justify-center">
        <LoadingSpinner />
      </div>
    ),
    children: [
      {
        element: <AuthLayout />,
        children: [
          { path: 'login', element: <LoginPage /> },
          { path: 'register', element: <RegisterPage /> },
        ],
      },
      {
        element: <RequireAuth />,
        children: [
          // Konva only loads with the labeling workspace, which keeps the rest of the app light.
          {
            path: 'clips/:id',
            lazy: () => import('@/pages/ClipPage').then((m) => ({ Component: m.ClipPage })),
          },
          {
            element: <AppLayout />,
            children: [
              { index: true, element: <VideosPage /> },
              { path: 'videos/:id', element: <VideoPage /> },
              { path: 'account', element: <AccountPage /> },
              { path: 'leaderboard', element: <LeaderboardPage /> },
              {
                element: <RequirePermission permission={Permission.MANAGE_LABEL_TYPES} />,
                children: [{ path: 'labels', element: <LabelsPage /> }],
              },
              {
                element: <RequirePermission permission={Permission.MANAGE_EXPORTS} />,
                children: [{ path: 'exports', element: <ExportsPage /> }],
              },
              {
                element: <RequirePermission permission={Permission.MANAGE_USERS} />,
                children: [{ path: 'users', element: <UsersPage /> }],
              },
              { path: '*', element: <NotFound /> },
            ],
          },
        ],
      },
    ],
  },
])
