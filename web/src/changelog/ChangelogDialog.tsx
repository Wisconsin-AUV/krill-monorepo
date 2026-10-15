import { ArrowTopRightOnSquareIcon, XMarkIcon } from '@heroicons/react/16/solid'
import { useEffect } from 'react'
import { Button } from '@/components/ui/Button'
import { QueryError } from '@/components/ui/Callout'
import { Dialog, DialogBody, DialogTitle } from '@/components/ui/Dialog'
import LoadingSpinner from '@/components/ui/LoadingSpinner'
import { REPO_URL } from '@/lib/links'
import { useReleases } from './releases'
import { ReleaseNotes } from './ReleaseNotes'
import { useChangelog } from './store'

const anchor = (version: string) => `release-${version.replaceAll('.', '-')}`

export function ChangelogDialog() {
  const { open, focus, close, markSeen } = useChangelog()
  const { data, isPending, error } = useReleases(open)

  useEffect(() => {
    if (!open || !data?.length) return
    const latest = data.find((r) => r.version !== 'Unreleased')
    if (latest) markSeen(latest.version)
    if (focus) document.getElementById(anchor(focus))?.scrollIntoView({ block: 'start' })
  }, [open, data, focus, markSeen])

  return (
    <Dialog open={open} onClose={close} size="2xl">
      <div className="flex items-center justify-between gap-4">
        <DialogTitle>Changelog</DialogTitle>
        <div className="-my-1.5 -mr-2 flex items-center">
          <Button
            plain
            to={`${REPO_URL}blob/develop/CHANGELOG.md`}
            target="_blank"
            rel="noreferrer"
            aria-label="Open on GitHub"
          >
            <ArrowTopRightOnSquareIcon data-slot="icon" />
          </Button>
          <Button plain onClick={close} aria-label="Close">
            <XMarkIcon data-slot="icon" />
          </Button>
        </div>
      </div>
      <DialogBody>
        {isPending ? (
          <div className="flex justify-center py-16">
            <LoadingSpinner />
          </div>
        ) : error ? (
          <QueryError title="Could not load the changelog" error={error} />
        ) : (
          <div className="divide-y divide-zinc-950/5 dark:divide-white/5">
            {data.map((r, i) => (
              <div key={r.version} className="py-8 first:pt-0 last:pb-0">
                <ReleaseNotes
                  release={r}
                  id={anchor(r.version)}
                  expanded={focus ? r.version === focus : i === 0}
                />
              </div>
            ))}
          </div>
        )}
      </DialogBody>
    </Dialog>
  )
}
