import * as Headless from '@headlessui/react'
import { ChevronRightIcon } from '@heroicons/react/16/solid'
import { InlineMarkdown } from './InlineMarkdown'
import { formatReleaseDate, type Release } from './parse'

// Fixes are the long tail, so they start folded like Linear's.
const folded = new Set(['Fixed', 'Removed', 'Deprecated', 'Security'])

// Only the release someone is likely there for starts open; the rest are
// headings to scan.
export function ReleaseNotes({
  release,
  id,
  expanded = false,
}: {
  release: Release
  id?: string
  expanded?: boolean
}) {
  return (
    <article id={id} className="scroll-mt-6">
      <div className="flex items-baseline justify-between gap-4">
        <h2 className="text-xl/8 font-semibold text-zinc-950 sm:text-lg/7 dark:text-white">
          {release.version}
        </h2>
        {release.date && (
          <time
            dateTime={release.date}
            className="text-sm/6 text-zinc-500 tabular-nums dark:text-zinc-400"
          >
            {formatReleaseDate(release.date)}
          </time>
        )}
      </div>
      <div className="mt-4 space-y-1">
        {release.sections.map((s) => (
          <Headless.Disclosure key={s.title} defaultOpen={expanded && !folded.has(s.title)}>
            <Headless.DisclosureButton className="group flex w-full items-center gap-2 rounded-lg py-1.5 text-left text-base/6 font-medium text-zinc-950 sm:text-sm/6 dark:text-white">
              <ChevronRightIcon className="size-4 fill-zinc-400 transition-transform duration-150 group-data-open:rotate-90 dark:fill-zinc-500" />
              {s.title}
              <span className="text-sm/6 font-normal text-zinc-400 tabular-nums sm:text-xs/6 dark:text-zinc-500">
                {s.items.length}
              </span>
            </Headless.DisclosureButton>
            <Headless.DisclosurePanel
              as="ul"
              className="mt-1 mb-3 list-disc space-y-2 pl-11 text-base/6 text-zinc-700 marker:text-zinc-300 sm:text-sm/6 dark:text-zinc-300 dark:marker:text-zinc-600"
            >
              {s.items.map((item) => (
                <li key={item}>
                  <InlineMarkdown text={item} />
                </li>
              ))}
            </Headless.DisclosurePanel>
          </Headless.Disclosure>
        ))}
      </div>
    </article>
  )
}
