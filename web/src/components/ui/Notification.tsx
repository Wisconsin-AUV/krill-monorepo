import {
  CheckCircleIcon,
  ExclamationTriangleIcon,
  InformationCircleIcon,
} from '@heroicons/react/24/outline'
import { XMarkIcon } from '@heroicons/react/20/solid'
import type { FlashMessage } from '@/store/useUIStore'

const icons = {
  success: <CheckCircleIcon className="size-6 text-emerald-500" aria-hidden="true" />,
  info: <InformationCircleIcon className="size-6 text-sky-500" aria-hidden="true" />,
  error: <ExclamationTriangleIcon className="size-6 text-red-500" aria-hidden="true" />,
}

export function Notification({ flash, onClose }: { flash: FlashMessage; onClose: () => void }) {
  return (
    <div
      role={flash.type === 'error' ? 'alert' : undefined}
      className="pointer-events-auto flex w-full max-w-sm items-start rounded-xl bg-white p-4 shadow-lg ring-1 ring-zinc-950/10 dark:bg-zinc-800 dark:ring-white/10"
    >
      <div className="shrink-0">{icons[flash.type]}</div>
      <div className="ml-3 w-0 flex-1 pt-0.5">
        <p className="text-sm font-medium text-zinc-950 dark:text-white">{flash.title}</p>
        {flash.message && (
          <p className="mt-1 text-sm text-zinc-600 dark:text-zinc-300">{flash.message}</p>
        )}
      </div>
      <button
        type="button"
        onClick={onClose}
        className="ml-4 shrink-0 rounded-md text-zinc-500 hover:text-zinc-700 focus:outline-2 focus:outline-offset-2 focus:outline-sky-500 dark:text-zinc-400 dark:hover:text-zinc-200"
      >
        <span className="sr-only">Close</span>
        <XMarkIcon className="size-5" aria-hidden="true" />
      </button>
    </div>
  )
}
