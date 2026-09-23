import { Fragment, useEffect } from 'react'
import { Transition } from '@headlessui/react'
import { Button } from './Button'

interface UnsavedChangesBarProps {
  isDirty: boolean
  isSubmitting: boolean
  onDiscard: () => void
}

export function UnsavedChangesBar({ isDirty, isSubmitting, onDiscard }: UnsavedChangesBarProps) {
  useEffect(() => {
    if (!isDirty) return
    const handler = (e: BeforeUnloadEvent) => {
      e.preventDefault()
    }
    window.addEventListener('beforeunload', handler)
    return () => window.removeEventListener('beforeunload', handler)
  }, [isDirty])

  const saveButton = (
    <Button type="submit" disabled={isSubmitting}>
      {isSubmitting ? (
        <span className="flex items-center gap-2">
          <svg
            className="h-4 w-4 animate-spin"
            xmlns="http://www.w3.org/2000/svg"
            fill="none"
            viewBox="0 0 24 24"
          >
            <circle
              className="opacity-25"
              cx="12"
              cy="12"
              r="10"
              stroke="currentColor"
              strokeWidth="4"
            />
            <path
              className="opacity-75"
              fill="currentColor"
              d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
            />
          </svg>
          Saving…
        </span>
      ) : (
        'Save Changes'
      )}
    </Button>
  )

  return (
    <div
      className={`flex items-center justify-end gap-3 rounded-lg border px-4 py-3 transition-colors duration-200 ${
        isDirty ? 'border-zinc-950/10 dark:border-white/10' : 'border-transparent'
      }`}
    >
      <Transition
        show={isDirty && !isSubmitting}
        as={Fragment}
        enter="transition-opacity ease-out duration-200"
        enterFrom="opacity-0"
        enterTo="opacity-100"
        leave="transition-opacity ease-in duration-150"
        leaveFrom="opacity-100"
        leaveTo="opacity-0"
      >
        <div className="flex items-center gap-3">
          <p className="text-sm text-zinc-600 dark:text-zinc-400">
            Careful - you have unsaved changes!
          </p>
          <Button outline type="button" disabled={isSubmitting} onClick={onDiscard}>
            Discard
          </Button>
        </div>
      </Transition>
      {saveButton}
    </div>
  )
}
