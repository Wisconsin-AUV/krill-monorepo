import { createBrowserRouter } from 'react-router'
import { AppLayout } from '@/layouts/AppLayout'
import { ExportsPage } from '@/pages/ExportsPage'
import { LabelsPage } from '@/pages/LabelsPage'
import { NotFound } from '@/pages/NotFound'
import { VideoPage } from '@/pages/VideoPage'
import { VideosPage } from '@/pages/VideosPage'

export const router = createBrowserRouter([
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
])
