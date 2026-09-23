import React, { Fragment } from 'react'
import { useUIStore } from '@/store/useUIStore'
import Notification from './Notification'

interface FlashMessageRenderProps {
  byKey: string
  className?: string
}

const FlashMessageRender: React.FC<FlashMessageRenderProps> = ({ byKey }) => {
  const flashes = useUIStore((s) => s.flashes)
  const filteredFlashes = flashes.filter((flash) => flash.key === byKey)

  if (filteredFlashes.length === 0) {
    return null
  }

  return (
    <div
      aria-live="assertive"
      className="pointer-events-none fixed inset-0 z-100 flex items-end px-4 py-6 sm:items-start sm:p-6"
    >
      <div className="flex w-full flex-col items-center space-y-4 sm:items-end">
        {filteredFlashes.map((flash, index) => (
          <Fragment key={flash.id ?? `${flash.key}-${index}`}>
            {index > 0 && <div className="mt-2"></div>}
            <Notification type={flash.type} title={flash.title}>
              {flash.message}
            </Notification>
          </Fragment>
        ))}
      </div>
    </div>
  )
}

export default FlashMessageRender
