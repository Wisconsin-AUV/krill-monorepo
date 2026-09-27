import { ChevronLeftIcon, ChevronRightIcon } from '@heroicons/react/16/solid'
import { useState } from 'react'
import type { LabelType } from '@/gen/krill/v1/label_pb'

export function LabelGuide({ type }: { type: LabelType }) {
  const [index, setIndex] = useState(0)
  const examples = type.examples
  const shown = Math.min(index, examples.length - 1)
  const example = examples[shown]
  const go = (delta: number) => setIndex((shown + delta + examples.length) % examples.length)

  if (!type.title && !type.description && !type.guideline && examples.length === 0) return null

  return (
    <section className="max-h-[55%] shrink-0 space-y-2 overflow-y-auto border-t border-zinc-950/10 p-3 dark:border-white/10">
      <h3 className="text-sm/5 font-semibold text-zinc-950 dark:text-white">
        {type.title || type.name}
      </h3>
      {type.description && (
        <p className="text-xs/5 text-zinc-600 dark:text-zinc-400">{type.description}</p>
      )}
      {type.guideline && (
        <p className="text-xs/5 whitespace-pre-line text-zinc-700 dark:text-zinc-300">
          {type.guideline}
        </p>
      )}
      {example && (
        <figure>
          <div className="relative overflow-hidden rounded-md bg-zinc-950">
            <a href={example.url} target="_blank" rel="noreferrer">
              <img
                src={example.url}
                alt={example.caption || `${type.name} example`}
                className="aspect-video w-full object-contain"
              />
            </a>
            {examples.length > 1 && (
              <>
                <button
                  type="button"
                  aria-label="Previous example"
                  onClick={() => go(-1)}
                  className="absolute top-1/2 left-1 -translate-y-1/2 rounded-full bg-zinc-950/60 p-1 text-white hover:bg-zinc-950/80"
                >
                  <ChevronLeftIcon className="size-4" />
                </button>
                <button
                  type="button"
                  aria-label="Next example"
                  onClick={() => go(1)}
                  className="absolute top-1/2 right-1 -translate-y-1/2 rounded-full bg-zinc-950/60 p-1 text-white hover:bg-zinc-950/80"
                >
                  <ChevronRightIcon className="size-4" />
                </button>
              </>
            )}
          </div>
          <figcaption className="mt-1 flex gap-2 text-xs/5 text-zinc-500 dark:text-zinc-400">
            <span className="min-w-0 flex-1">{example.caption}</span>
            {examples.length > 1 && (
              <span className="tabular-nums">
                {shown + 1}/{examples.length}
              </span>
            )}
          </figcaption>
        </figure>
      )}
    </section>
  )
}
