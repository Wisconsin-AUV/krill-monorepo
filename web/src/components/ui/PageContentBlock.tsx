import React, { useEffect } from 'react'
import { clsx } from 'clsx'

export interface PageContentBlockProps {
  title?: string
  className?: string
  width?: string
  children?: React.ReactNode
}
const PageContentBlock: React.FC<PageContentBlockProps> = ({
  title,
  className,
  width = 'max-w-6xl p-6 lg:p-10',
  children,
}) => {
  useEffect(() => {
    if (title) {
      document.title = title
    }
  }, [title])

  return (
    <>
      <div className={clsx('mx-auto', width, className)}>{children}</div>
    </>
  )
}

export default PageContentBlock
