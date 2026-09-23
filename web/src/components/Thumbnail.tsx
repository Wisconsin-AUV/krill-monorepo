import { PhotoIcon } from '@heroicons/react/24/outline'
import { clsx } from 'clsx'
import { useState } from 'react'

export function Thumbnail({
  src,
  alt = '',
  className,
}: {
  src?: string
  alt?: string
  className?: string
}) {
  const [failed, setFailed] = useState(false)
  return (
    <div
      className={clsx(
        className,
        'relative aspect-video overflow-hidden rounded-md bg-zinc-100 ring-1 ring-zinc-950/5 ring-inset dark:bg-zinc-800 dark:ring-white/10',
      )}
    >
      {src && !failed ? (
        <img
          src={src}
          alt={alt}
          loading="lazy"
          className="size-full object-cover"
          onError={() => setFailed(true)}
        />
      ) : (
        <PhotoIcon
          className="absolute inset-0 m-auto size-1/3 text-zinc-300 dark:text-zinc-600"
          aria-hidden="true"
        />
      )}
    </div>
  )
}
