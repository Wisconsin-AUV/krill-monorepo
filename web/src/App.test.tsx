import { createRouterTransport } from '@connectrpc/connect'
import { render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'
import { AuthService } from '@/gen/krill/v1/auth_pb'
import { HealthService } from '@/gen/krill/v1/health_pb'
import { Role } from '@/gen/krill/v1/user_pb'
import { VideoService } from '@/gen/krill/v1/video_pb'
import { queryClient } from '@/lib/queryClient'
import { router } from '@/router'
import App from './App'

function transport(role?: Role) {
  return createRouterTransport(({ service }) => {
    service(AuthService, {
      getSession: () => ({
        user: role ? { id: '1', name: 'Ada Lovelace', username: 'ada', role } : undefined,
        signupEnabled: true,
      }),
    })
    service(HealthService, { check: () => ({ version: 'test' }) })
    service(VideoService, { listVideos: () => ({ videos: [] }) })
  })
}

describe('App', () => {
  afterEach(async () => {
    queryClient.clear()
    await router.navigate('/')
  })

  it('sends signed-out users to the login page', async () => {
    render(<App transport={transport()} />)
    expect(await screen.findByRole('heading', { name: 'Sign in' })).toBeInTheDocument()
  })

  it('shows labelers only the pages they can use', async () => {
    render(<App transport={transport(Role.LABELER)} />)
    expect(await screen.findAllByText('Videos')).not.toHaveLength(0)
    expect(screen.queryByText('Exports')).not.toBeInTheDocument()
    expect(screen.queryByText('Upload videos')).not.toBeInTheDocument()
  })

  it('shows admins every page', async () => {
    render(<App transport={transport(Role.ADMIN)} />)
    expect(await screen.findAllByText('Users')).not.toHaveLength(0)
    expect(screen.getAllByText('Exports')).not.toHaveLength(0)
  })
})
