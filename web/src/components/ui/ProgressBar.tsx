import React, { useEffect, useRef, Fragment } from 'react'
import { useProgressStore } from '@/store/useProgressStore'
import { randomInt } from '@/lib/helpers'
import { Transition } from '@headlessui/react'

type Timer = ReturnType<typeof setTimeout>

const ProgressBar: React.FC = () => {
  const interval = useRef<Timer | null>(null)
  const timeout = useRef<Timer | null>(null)
  const progress = useProgressStore((s) => s.progress)
  const continuous = useProgressStore((s) => s.continuous)
  const setProgress = useProgressStore((s) => s.setProgress)
  const visible = (progress || 0) > 0

  useEffect(() => {
    return () => {
      if (timeout.current) clearTimeout(timeout.current)
      if (interval.current) clearInterval(interval.current)
    }
  }, [])

  useEffect(() => {
    if (progress === 100) {
      timeout.current = setTimeout(() => setProgress(undefined), 500)
    }
  }, [progress, setProgress])

  useEffect(() => {
    if (!continuous) {
      if (interval.current) clearInterval(interval.current)
      return
    }

    if (!progress || progress === 0) {
      setProgress(randomInt(20, 30))
    }
  }, [continuous, progress, setProgress])

  useEffect(() => {
    if (continuous) {
      if (interval.current) clearInterval(interval.current)
      if ((progress || 0) >= 90) {
        setProgress(90)
      } else {
        interval.current = setTimeout(() => setProgress((progress || 0) + randomInt(1, 5)), 500)
      }
    }
  }, [progress, continuous, setProgress])

  return (
    <div className="fixed z-50 h-[2px] w-full">
      <Transition
        appear
        unmount
        as={Fragment}
        show={visible}
        enter="transition-opacity duration-150"
        enterFrom="opacity-0"
        enterTo="opacity-100"
        leave="transition-opacity duration-150"
        leaveFrom="opacity-100"
        leaveTo="opacity-0"
      >
        <div
          className="h-full bg-indigo-600 shadow-[0_-2px_8px_2px] shadow-indigo-600 transition-all duration-[250ms] ease-in-out"
          style={{ width: progress === undefined ? '100%' : `${progress}%` }}
        />
      </Transition>
    </div>
  )
}

export default ProgressBar
