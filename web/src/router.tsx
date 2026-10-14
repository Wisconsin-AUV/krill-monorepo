import { createBrowserRouter, Navigate } from 'react-router'
import LoadingSpinner from '@/components/ui/LoadingSpinner'
import { Permission } from '@/gen/krill/v1/user_pb'
import { AppChrome, AppLayout } from '@/layouts/AppLayout'
import { AuthLayout } from '@/layouts/AuthLayout'
import { RequireAuth, RequirePermission } from '@/layouts/RequireAuth'
import { VerifyLayout } from '@/layouts/VerifyLayout'
import { AccountPage } from '@/pages/AccountPage'
import { AttributesPage } from '@/pages/AttributesPage'
import { ExportsPage } from '@/pages/ExportsPage'
import { GoldPage } from '@/pages/GoldPage'
import { HomePage } from '@/pages/HomePage'
import { LabelsPage } from '@/pages/LabelsPage'
import { LeaderboardPage } from '@/pages/LeaderboardPage'
import { LoginPage } from '@/pages/LoginPage'
import { NotFound } from '@/pages/NotFound'
import { ProfilePage } from '@/pages/ProfilePage'
import { RegisterPage } from '@/pages/RegisterPage'
import { ResetPasswordPage } from '@/pages/ResetPasswordPage'
import { FlaggedPage, ReviewPage } from '@/pages/ReviewPage'
import { VideoPage } from '@/pages/VideoPage'
import { VerifyEmailPage } from '@/pages/VerifyEmailPage'
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
          { path: 'reset-password', element: <ResetPasswordPage /> },
        ],
      },
      {
        element: <VerifyLayout />,
        children: [{ path: 'verify', element: <VerifyEmailPage /> }],
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
            path: 'check/:id',
            lazy: () =>
              import('@/pages/GoldCheckPage').then((m) => ({ Component: m.GoldCheckPage })),
          },
          {
            path: 'gold/attempts/:frameId/:userId',
            lazy: () =>
              import('@/pages/GoldCheckPage').then((m) => ({ Component: m.GoldAttemptPage })),
          },
          // Only admins see the admin panel, so nobody else downloads it.
          {
            path: 'admin',
            lazy: () => import('@/admin/AdminLayout').then((m) => ({ Component: m.AdminLayout })),
            children: [
              {
                element: <RequirePermission permission={Permission.MANAGE_SYSTEM} />,
                children: [
                  {
                    index: true,
                    lazy: () =>
                      import('@/admin/OverviewPage').then((m) => ({ Component: m.OverviewPage })),
                  },
                  {
                    path: 'settings',
                    lazy: () =>
                      import('@/admin/SettingsPage').then((m) => ({ Component: m.SettingsPage })),
                  },
                  {
                    path: 'compute',
                    lazy: () =>
                      import('@/admin/ComputePage').then((m) => ({ Component: m.ComputePage })),
                  },
                  {
                    path: 'audit',
                    lazy: () =>
                      import('@/admin/AuditPage').then((m) => ({ Component: m.AuditPage })),
                  },
                  {
                    path: 'emails',
                    lazy: () =>
                      import('@/admin/EmailsPage').then((m) => ({ Component: m.EmailsPage })),
                  },
                  {
                    path: 'labelers',
                    lazy: () =>
                      import('@/admin/LabelersPage').then((m) => ({ Component: m.LabelersPage })),
                  },
                  {
                    path: 'coverage',
                    lazy: () =>
                      import('@/admin/CoveragePage').then((m) => ({ Component: m.CoveragePage })),
                  },
                  {
                    path: 'stuck',
                    lazy: () =>
                      import('@/admin/StuckPage').then((m) => ({ Component: m.StuckPage })),
                  },
                  {
                    path: 'jobs',
                    lazy: () => import('@/admin/JobsPage').then((m) => ({ Component: m.JobsPage })),
                  },
                  {
                    path: 'worker-tokens',
                    lazy: () =>
                      import('@/admin/WorkerTokensPage').then((m) => ({
                        Component: m.WorkerTokensPage,
                      })),
                  },
                ],
              },
              {
                element: <RequirePermission permission={Permission.MANAGE_USERS} />,
                children: [
                  {
                    path: 'users',
                    lazy: () =>
                      import('@/admin/UsersPage').then((m) => ({ Component: m.UsersPage })),
                  },
                  {
                    path: 'users/:id',
                    lazy: () =>
                      import('@/admin/UserDetailPage').then((m) => ({
                        Component: m.UserDetailPage,
                      })),
                  },
                  {
                    path: 'invites',
                    lazy: () =>
                      import('@/admin/InvitesPage').then((m) => ({ Component: m.InvitesPage })),
                  },
                  {
                    path: 'sessions',
                    lazy: () =>
                      import('@/admin/SessionsPage').then((m) => ({ Component: m.SessionsPage })),
                  },
                ],
              },
              { path: '*', element: <NotFound /> },
            ],
          },
          {
            element: <AppLayout />,
            children: [
              { index: true, element: <HomePage /> },
              { path: 'videos', element: <VideosPage /> },
              { path: 'videos/:id', element: <VideoPage /> },
              { path: 'account', element: <AccountPage /> },
              { path: 'attributes', element: <AttributesPage /> },
              { path: 'leaderboard', element: <LeaderboardPage /> },
              { path: 'people/:username', element: <ProfilePage /> },
              {
                element: <RequirePermission permission={Permission.MANAGE_LABEL_TYPES} />,
                children: [{ path: 'labels', element: <LabelsPage /> }],
              },
              {
                element: <RequirePermission permission={Permission.MANAGE_EXPORTS} />,
                children: [{ path: 'exports', element: <ExportsPage /> }],
              },
              {
                element: <RequirePermission permission={Permission.REVIEW_LABELS} />,
                children: [
                  {
                    path: 'review',
                    element: <ReviewPage />,
                    children: [
                      { index: true, element: <FlaggedPage /> },
                      { path: 'gold', element: <GoldPage /> },
                    ],
                  },
                  { path: 'gold', element: <Navigate to="/review/gold" replace /> },
                ],
              },
              { path: 'users', element: <Navigate to="/admin/users" replace /> },
              { path: '*', element: <NotFound /> },
            ],
          },
        ],
      },
    ],
  },
])
