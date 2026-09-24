import { Code, ConnectError, createRouterTransport } from '@connectrpc/connect'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'
import { AuthService } from '@/gen/krill/v1/auth_pb'
import { HealthService } from '@/gen/krill/v1/health_pb'
import { Permission, Role } from '@/gen/krill/v1/user_pb'
import { VideoService } from '@/gen/krill/v1/video_pb'
import { queryClient } from '@/lib/queryClient'
import { router } from '@/router'
import App from './App'

function transport(role?: Role) {
  return createRouterTransport(({ service }) => {
    service(AuthService, {
      getSession: () => ({
        user: role
          ? {
              id: '1',
              name: 'Ada Lovelace',
              username: 'ada',
              role,
              permissions: [Permission.LABEL],
            }
          : undefined,
        signupEnabled: true,
      }),
      login: () => {
        throw new ConnectError('incorrect username or password', Code.Unauthenticated)
      },
    })
    service(HealthService, { check: () => ({ version: 'test' }) })
    service(VideoService, { listVideos: () => ({ videos: [] }) })
  })
}

describe('App', () => {
  afterEach(async () => {
    cleanup()
    queryClient.clear()
    await router.navigate('/')
  })

  it('sends signed-out users to the login page', async () => {
    render(<App transport={transport()} />)
    expect(await screen.findByRole('heading', { name: 'Sign in to Krill' })).toBeInTheDocument()
  })

  it('shows labelers only the pages they can use', async () => {
    render(<App transport={transport(Role.LABELER)} />)
    expect(await screen.findAllByText('Videos')).not.toHaveLength(0)
    expect(screen.queryByText('Exports')).not.toBeInTheDocument()
    expect(screen.queryByText('Upload videos')).not.toBeInTheDocument()
  })

  it('asks for the password after the username and shows a wrong password', async () => {
    render(<App transport={transport()} />)
    const login = await screen.findByRole('textbox', { name: 'Username or email' })
    expect(screen.queryByLabelText('Password')).not.toBeInTheDocument()

    fireEvent.change(login, { target: { value: 'ada' } })
    fireEvent.click(screen.getByRole('button', { name: 'Continue' }))
    const password = await screen.findByLabelText('Password')
    expect(login).toHaveAttribute('readonly')

    fireEvent.change(password, { target: { value: 'wrong' } })
    fireEvent.click(screen.getByRole('button', { name: 'Sign in' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('incorrect username or password')
    expect(password).toHaveAttribute('aria-invalid', 'true')

    fireEvent.change(password, { target: { value: 'wrong again' } })
    expect(password).not.toHaveAttribute('aria-invalid')
  })
})
