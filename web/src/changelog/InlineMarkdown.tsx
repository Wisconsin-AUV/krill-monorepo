import { Fragment, type ReactNode } from 'react'
import { Code } from '@/components/ui/Text'

const token = /`([^`]+)`|\[([^\]]+)\]\((https?:\/\/[^)\s]+)\)/g

// Changelog entries only use inline code and links.
export function InlineMarkdown({ text }: { text: string }) {
  const out: ReactNode[] = []
  let last = 0
  for (const m of text.matchAll(token)) {
    out.push(text.slice(last, m.index))
    if (m[1] !== undefined) {
      out.push(<Code key={m.index}>{m[1]}</Code>)
    } else {
      out.push(
        <a
          key={m.index}
          href={m[3]}
          target="_blank"
          rel="noreferrer"
          className="text-zinc-950 underline decoration-zinc-950/50 hover:decoration-zinc-950 dark:text-white dark:decoration-white/50 dark:hover:decoration-white"
        >
          {m[2]}
        </a>,
      )
    }
    last = m.index + m[0].length
  }
  out.push(text.slice(last))
  return <Fragment>{out}</Fragment>
}
