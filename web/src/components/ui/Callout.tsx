import { ExclamationCircleIcon, ExclamationTriangleIcon } from '@heroicons/react/20/solid'
import { clsx } from 'clsx'
import { errorMessage } from '@/lib/errors'

const styles = {
  error: 'bg-red-500/10 text-red-700 dark:bg-red-500/15 dark:text-red-400',
  warning: 'bg-amber-500/10 text-amber-800 dark:bg-amber-400/10 dark:text-amber-200',
}

export function Callout({
  type = 'error',
  title,
  action,
  className,
  children,
}: {
  type?: keyof typeof styles
  title?: string
  action?: React.ReactNode
  className?: string
  children: React.ReactNode
}) {
  const Icon = type === 'error' ? ExclamationCircleIcon : ExclamationTriangleIcon
  return (
    <div
      role={type === 'error' ? 'alert' : undefined}
      className={clsx(
        className,
        'flex items-start gap-3 rounded-lg px-4 py-3 text-sm/6',
        styles[type],
      )}
    >
      <Icon className="mt-0.5 size-5 shrink-0" aria-hidden="true" />
      <div className="min-w-0 flex-1 break-words">
        {title && <p className="font-medium">{title}</p>}
        {children}
      </div>
      {action}
    </div>
  )
}

export function FormError({ error, className }: { error: unknown; className?: string }) {
  if (!error) return null
  return <Callout className={className}>{errorMessage(error)}</Callout>
}

export function QueryError({
  title,
  error,
  className,
}: {
  title: string
  error: unknown
  className?: string
}) {
  return (
    <Callout title={title} className={className}>
      {errorMessage(error)}
    </Callout>
  )
}
