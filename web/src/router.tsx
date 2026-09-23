import { createBrowserRouter } from 'react-router'
import { AppLayout } from '@/layouts/AppLayout'
import { NotFound } from '@/pages/NotFound'

export const router = createBrowserRouter([
  {
    element: <AppLayout />,
    children: [{ path: '*', element: <NotFound /> }],
  },
])
