import { REPO_URL, WAUV_NAME, WAUV_URL } from '@/lib/links'

const linkClass =
  'text-zinc-600 underline decoration-zinc-950/20 hover:text-zinc-950 dark:text-zinc-400 dark:decoration-white/20 dark:hover:text-white'

export function Credit({ team }: { team: string }) {
  const krill = (
    <a href={REPO_URL} target="_blank" rel="noreferrer" className={linkClass}>
      Krill
    </a>
  )
  const wauv = (
    <a href={WAUV_URL} target="_blank" rel="noreferrer" className={linkClass}>
      {WAUV_NAME}
    </a>
  )
  const ours = !team || team === WAUV_NAME
  return (
    <div className="space-y-0.5 text-xs/5 text-zinc-500 dark:text-zinc-500">
      <p>
        {ours ? (
          <>
            {krill} is a labeling tool by {wauv}.
          </>
        ) : (
          <>
            {krill} for {team}. Made by {wauv}.
          </>
        )}
      </p>
      <p>For {ours ? 'team' : team} members only. Not for public use.</p>
    </div>
  )
}
