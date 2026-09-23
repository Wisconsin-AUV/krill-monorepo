import { createBrowserRouter } from 'react-router'
import LoadingSpinner from '@/components/ui/LoadingSpinner'
import { AppChrome, AppLayout } from '@/layouts/AppLayout'
import { ExportsPage } from '@/pages/ExportsPage'
import { LabelsPage } from '@/pages/LabelsPage'
import { NotFound } from '@/pages/NotFound'
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
          { path: 'labels', element: <LabelsPage /> },
          { path: 'exports', element: <ExportsPage /> },
          { path: '*', element: <NotFound /> },
        ],
      },
    ],
  },
])
