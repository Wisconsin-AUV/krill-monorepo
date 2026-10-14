import { Code, ConnectError, createRouterTransport } from '@connectrpc/connect'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'
import { AuthService } from '@/gen/krill/v1/auth_pb'
import { HealthService } from '@/gen/krill/v1/health_pb'
import { QueueService } from '@/gen/krill/v1/queue_pb'
import { Permission, Role } from '@/gen/krill/v1/user_pb'
import { VideoService } from '@/gen/krill/v1/video_pb'
import { queryClient } from '@/lib/queryClient'
import { router } from '@/router'
import App from './App'

function transport(role?: Role, { emailVerified = true } = {}) {
  return createRouterTransport(({ service }) => {
    service(AuthService, {
      getSession: () => ({
        user: role
          ? {
              id: '1',
              name: 'Ada Lovelace',
              username: 'ada',
              email: 'ada@wisc.edu',
              role,
              permissions: [Permission.LABEL],
              emailVerified,
            }
          : undefined,
        signupEnabled: true,
        emailVerificationRequired: true,
        emailCodesEnabled: true,
      }),
      sendVerificationCode: () => ({}),
      requestSignInCode: () => ({}),
      signInWithCode: (req) => {
        if (req.code !== '123456') {
          throw new ConnectError('that code is incorrect or has expired', Code.InvalidArgument)
        }
        return {}
      },
      login: () => {
        throw new ConnectError('incorrect username or password', Code.Unauthenticated)
      },
    })
    service(HealthService, { check: () => ({ version: 'test' }) })
    service(VideoService, { listVideos: () => ({ videos: [] }) })
    service(QueueService, { getQueue: () => ({}) })
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
    expect(await screen.findAllByText('Home')).not.toHaveLength(0)
    expect(screen.queryByText('Videos')).not.toBeInTheDocument()
    expect(screen.queryByText('Exports')).not.toBeInTheDocument()
    expect(screen.queryByText('Upload videos')).not.toBeInTheDocument()
  })

  it('keeps unverified accounts on the verify page', async () => {
    await router.navigate('/videos')
    render(<App transport={transport(Role.ADMIN, { emailVerified: false })} />)
    expect(await screen.findByRole('heading', { name: 'Verify your email' })).toBeInTheDocument()
    expect(screen.getByText('ada@wisc.edu')).toBeInTheDocument()
    expect(screen.queryByText('Home')).not.toBeInTheDocument()
  })

  it('signs in with an emailed code', async () => {
    render(<App transport={transport()} />)
    fireEvent.change(await screen.findByRole('textbox', { name: 'Username or email' }), {
      target: { value: 'ada' },
    })
    fireEvent.click(screen.getByRole('button', { name: 'Continue' }))
    fireEvent.click(await screen.findByRole('button', { name: 'Email sign-in code' }))
    const code = await screen.findByRole('textbox', { name: 'Code' })

    fireEvent.change(code, { target: { value: '000000' } })
    expect(await screen.findByRole('alert')).toHaveTextContent(
      'That code is incorrect or has expired',
    )
    expect(code).toHaveValue('')
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
    expect(await screen.findByRole('alert')).toHaveTextContent('Incorrect username or password')
    expect(password).toHaveAttribute('aria-invalid', 'true')

    fireEvent.change(password, { target: { value: 'wrong again' } })
    expect(password).not.toHaveAttribute('aria-invalid')
  })
})
