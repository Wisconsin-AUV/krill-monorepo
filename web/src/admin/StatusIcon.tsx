import {
  CheckCircleIcon,
  ExclamationTriangleIcon,
  MinusCircleIcon,
  XCircleIcon,
} from '@heroicons/react/20/solid'
import { clsx } from 'clsx'
import { CheckStatus } from '@/gen/krill/v1/admin_pb'

const looks: Record<
  CheckStatus,
  { icon: typeof CheckCircleIcon; label: string; className: string }
> = {
  [CheckStatus.OK]: { icon: CheckCircleIcon, label: 'OK', className: 'text-emerald-500' },
  [CheckStatus.WARNING]: {
    icon: ExclamationTriangleIcon,
    label: 'Warning',
    className: 'text-amber-500',
  },
  [CheckStatus.ERROR]: { icon: XCircleIcon, label: 'Down', className: 'text-red-500' },
  [CheckStatus.OFF]: {
    icon: MinusCircleIcon,
    label: 'Off',
    className: 'text-zinc-400 dark:text-zinc-500',
  },
  [CheckStatus.UNSPECIFIED]: {
    icon: MinusCircleIcon,
    label: 'Unknown',
    className: 'text-zinc-400 dark:text-zinc-500',
  },
}

export function StatusIcon({ status, className }: { status: CheckStatus; className?: string }) {
  const { icon: Icon, label, className: color } = looks[status]
  return <Icon title={label} className={clsx(className, 'size-5', color)} />
}
