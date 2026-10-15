import { ConnectError } from '@connectrpc/connect'
import { useMutation, useQuery } from '@connectrpc/connect-query'
import { CheckCircleIcon, XMarkIcon } from '@heroicons/react/16/solid'
import { useState } from 'react'
import { useSearchParams } from 'react-router'
import { Button } from '@/components/ui/Button'
import { Callout } from '@/components/ui/Callout'
import { Code } from '@/components/ui/Text'
import {
  type GetSlackTestResultResponse,
  GetSlackTestResultResponseSchema,
  SettingsCheckService,
  SlackProblem,
} from '@/gen/krill/v1/settings_check_pb'
import { flash } from '@/lib/flash'

type Result = Pick<GetSlackTestResultResponse, 'problem' | 'detail' | 'redirectUrl'> &
  Partial<Pick<GetSlackTestResultResponse, 'teamId' | 'teamName' | 'expectedTeamId' | 'userName'>>

function problemText(r: Result): { title: string; body?: React.ReactNode } {
  switch (r.problem) {
    case SlackProblem.OFF:
      return { title: 'No client ID saved' }
    case SlackProblem.CLIENT_ID:
      return { title: 'Slack doesn’t recognize the client ID' }
    case SlackProblem.CLIENT_SECRET:
      return { title: 'Wrong client secret' }
    case SlackProblem.REDIRECT_URL:
      return r.redirectUrl.startsWith('https://')
        ? {
            title: 'Add this redirect URL under OAuth & Permissions in Slack',
            body: <Code>{r.redirectUrl}</Code>,
          }
        : { title: 'Slack needs an HTTPS public URL', body: <Code>{r.redirectUrl}</Code> }
    case SlackProblem.TEAM_ID:
      return { title: 'Workspace ID should look like T0123ABCD' }
    case SlackProblem.WRONG_TEAM:
      return {
        title: 'Signed in to a different workspace',
        body: (
          <>
            {r.teamName} is <Code>{r.teamId}</Code>, but the saved ID is{' '}
            <Code>{r.expectedTeamId}</Code>
          </>
        ),
      }
    case SlackProblem.CANCELLED:
      return { title: 'Sign-in cancelled' }
    case SlackProblem.EXPIRED:
      return { title: 'Test expired' }
    case SlackProblem.UNREACHABLE:
      return { title: 'Could not reach Slack', body: r.detail }
    default:
      return { title: 'Slack refused', body: <Code>{r.detail}</Code> }
  }
}

function ResultView({
  result,
  success,
  onDismiss,
}: {
  result: Result
  success: React.ReactNode
  onDismiss: () => void
}) {
  const dismiss = (
    <button type="button" onClick={onDismiss} aria-label="Dismiss" className="shrink-0">
      <XMarkIcon className="size-4 opacity-60 hover:opacity-100" />
    </button>
  )
  if (result.problem === SlackProblem.UNSPECIFIED) {
    return (
      <div className="flex items-start gap-3 rounded-lg bg-emerald-500/10 px-4 py-3 text-sm/6 text-emerald-800 dark:bg-emerald-400/10 dark:text-emerald-300">
        <CheckCircleIcon className="mt-1 size-4 shrink-0" />
        <div className="min-w-0 flex-1 break-words">{success}</div>
        {dismiss}
      </div>
    )
  }
  const { title, body } = problemText(result)
  return (
    <Callout title={title} action={dismiss}>
      {body}
    </Callout>
  )
}

function detailOf(err: unknown): Result | undefined {
  return err instanceof ConnectError
    ? err.findDetails(GetSlackTestResultResponseSchema)[0]
    : undefined
}

// Checks run against the saved settings, so they wait for unsaved edits.
export function SlackCheck({ disabled }: { disabled: boolean }) {
  const [params, setParams] = useSearchParams()
  const testId = params.get('slack_test') ?? ''
  const [checked, setChecked] = useState<Result | null>(null)

  const returned = useQuery(
    SettingsCheckService.method.getSlackTestResult,
    { testId },
    { enabled: testId !== '' },
  )
  const check = useMutation(SettingsCheckService.method.checkSlackCredentials, {
    onSuccess: setChecked,
    onError: (err) => flash.error('Could not check Slack', err),
  })
  const start = useMutation(SettingsCheckService.method.startSlackTest, {
    onSuccess: (res) => window.location.assign(res.authorizeUrl),
    onError: (err) => {
      const d = detailOf(err)
      if (d) setChecked(d)
      else flash.error('Could not start the Slack test', err)
    },
  })
  const clearReturned = () => {
    const next = new URLSearchParams(params)
    next.delete('slack_test')
    setParams(next, { replace: true })
  }

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap gap-3">
        <Button
          outline
          onClick={() => check.mutate({})}
          disabled={disabled || check.isPending}
          title={disabled ? 'Save your changes first' : undefined}
        >
          Check credentials
        </Button>
        <Button
          outline
          onClick={() => start.mutate({})}
          disabled={disabled || start.isPending}
          title={disabled ? 'Save your changes first' : undefined}
        >
          Sign in with Slack
        </Button>
      </div>
      {checked && (
        <ResultView
          result={checked}
          onDismiss={() => setChecked(null)}
          success={
            <>
              Client ID and secret accepted. Redirect URL: <Code>{checked.redirectUrl}</Code>
            </>
          }
        />
      )}
      {returned.data && (
        <ResultView
          result={returned.data}
          onDismiss={clearReturned}
          success={
            <>
              Signed in to {returned.data.teamName || returned.data.teamId}
              {returned.data.userName && ` as ${returned.data.userName}`}
            </>
          }
        />
      )}
    </div>
  )
}
