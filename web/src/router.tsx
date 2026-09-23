import { createBrowserRouter } from 'react-router'
import { AppLayout } from '@/layouts/AppLayout'
import { ClipPage } from '@/pages/ClipPage'
import { ExportsPage } from '@/pages/ExportsPage'
import { LabelsPage } from '@/pages/LabelsPage'
import { NotFound } from '@/pages/NotFound'
import { VideoPage } from '@/pages/VideoPage'
import { VideosPage } from '@/pages/VideosPage'

export const router = createBrowserRouter([
  { path: 'clips/:id', element: <ClipPage /> },
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
