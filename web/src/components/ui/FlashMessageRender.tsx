import { AnimatePresence, motion } from 'motion/react'
import { useUIStore } from '@/store/useUIStore'
import { Notification } from './Notification'

export function FlashMessageRender() {
  const flashes = useUIStore((s) => s.flashes)
  const removeFlash = useUIStore((s) => s.removeFlash)

  return (
    // Always mounted: screen readers skip messages in a live region that was just added.
    <div
      aria-live="polite"
      className="pointer-events-none fixed inset-0 z-100 flex flex-col items-center gap-4 px-4 py-6 sm:items-end sm:p-6"
    >
      <AnimatePresence>
        {flashes.map((flash) => (
          <motion.div
            key={flash.id}
            layout
            className="flex w-full max-w-sm"
            initial={{ opacity: 0, x: 8 }}
            animate={{ opacity: 1, x: 0 }}
            exit={{ opacity: 0 }}
            transition={{ duration: 0.2 }}
          >
            <Notification flash={flash} onClose={() => removeFlash(flash.id)} />
          </motion.div>
        ))}
      </AnimatePresence>
    </div>
  )
}
