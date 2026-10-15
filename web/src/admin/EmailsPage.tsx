import { useMutation, useQuery } from '@connectrpc/connect-query'
import { PaperAirplaneIcon } from '@heroicons/react/16/solid'
import { useState } from 'react'
import { useSearchParams } from 'react-router'
import { SegmentedTabs } from '@/components/SegmentedTabs'
import { Button } from '@/components/ui/Button'
import { QueryError } from '@/components/ui/Callout'
import { Heading } from '@/components/ui/Heading'
import LoadingSpinner from '@/components/ui/LoadingSpinner'
import PageContentBlock from '@/components/ui/PageContentBlock'
import { Select } from '@/components/ui/Select'
import { SettingsCheckService } from '@/gen/krill/v1/settings_check_pb'
import { flash } from '@/lib/flash'

const views = [
  { key: 'html', label: 'HTML' },
  { key: 'text', label: 'Text' },
] as const

export function EmailsPage() {
  const { data, isPending, error } = useQuery(SettingsCheckService.method.listEmailPreviews, {})
  const [params, setParams] = useSearchParams()
  const [view, setView] = useState<'html' | 'text'>('html')
  const emails = data?.emails ?? []
  const email = emails.find((e) => e.name === params.get('email')) ?? emails[0]

  const send = useMutation(SettingsCheckService.method.sendEmailPreview, {
    onSuccess: (res) =>
      res.logged
        ? flash.info('Email written to the API log')
        : flash.success('Email sent', `Check ${res.to}.`),
    onError: (err) => flash.error('Could not send the email', err),
  })

  return (
    <PageContentBlock title="Emails · Admin · Krill">
      <Heading>Emails</Heading>
      {isPending ? (
        <div className="flex justify-center py-16">
          <LoadingSpinner />
        </div>
      ) : error ? (
        <QueryError title="Could not render emails" error={error} className="mt-8" />
      ) : email ? (
        <>
          <div className="mt-8 flex flex-wrap items-center gap-3">
            <Select
              aria-label="Email"
              value={email.name}
              onChange={(e) => setParams({ email: e.target.value }, { replace: true })}
              className="sm:w-56"
            >
              {emails.map((e) => (
                <option key={e.name} value={e.name}>
                  {e.label}
                </option>
              ))}
            </Select>
            <SegmentedTabs
              options={views}
              value={view}
              onChange={(k) => setView(k === 'text' ? 'text' : 'html')}
            />
            <Button
              outline
              className="ml-auto"
              onClick={() => send.mutate({ name: email.name })}
              disabled={send.isPending}
            >
              <PaperAirplaneIcon data-slot="icon" />
              Send to me
            </Button>
          </div>
          <p className="mt-6 text-sm/6 text-zinc-500 dark:text-zinc-400">
            Subject{' '}
            <span className="font-medium text-zinc-950 dark:text-white">{email.subject}</span>
          </p>
          <div className="mt-3 overflow-hidden rounded-xl ring-1 ring-zinc-950/10 dark:ring-white/10">
            {view === 'html' ? (
              // No sandbox permissions: the email can't run scripts or reach the app.
              <iframe
                title={email.label}
                sandbox=""
                srcDoc={email.html}
                className="block h-[42rem] w-full bg-white"
              />
            ) : (
              <pre className="max-h-[42rem] overflow-auto bg-white p-6 font-mono text-sm/6 whitespace-pre-wrap text-zinc-950 dark:bg-zinc-900 dark:text-zinc-100">
                {email.text}
              </pre>
            )}
          </div>
        </>
      ) : null}
    </PageContentBlock>
  )
}
