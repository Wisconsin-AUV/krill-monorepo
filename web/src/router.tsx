import { createBrowserRouter } from 'react-router'
import { AppLayout } from '@/layouts/AppLayout'
import { NotFound } from '@/pages/NotFound'
import { VideoPage } from '@/pages/VideoPage'
import { VideosPage } from '@/pages/VideosPage'

export const router = createBrowserRouter([
  {
    element: <AppLayout />,
    children: [
      { index: true, element: <VideosPage /> },
      { path: 'videos/:id', element: <VideoPage /> },
      { path: '*', element: <NotFound /> },
    ],
  },
])
