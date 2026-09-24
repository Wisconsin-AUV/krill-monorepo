import { ExclamationCircleIcon } from '@heroicons/react/20/solid'

export function FormError({ message }: { message: string | null }) {
  if (!message) return null
  return (
    <p
      role="alert"
      className="flex items-center gap-3 rounded-lg bg-red-500/10 px-4 py-3 text-sm/6 text-red-700 dark:bg-red-500/15 dark:text-red-400"
    >
      <ExclamationCircleIcon className="size-5 shrink-0" />
      {message}
    </p>
  )
}
