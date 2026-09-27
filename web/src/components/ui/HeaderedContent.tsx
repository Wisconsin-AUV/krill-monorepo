import * as React from 'react'
import { clsx } from 'clsx'
import { Heading } from './Heading'
import { Text } from './Text'

const HeaderedContent = ({
  title,
  description,
  children,
  className,
}: {
  title: string
  description: string
  children?: React.ReactNode
  className?: string
}) => (
  <>
    <div className={clsx('flex grid-cols-7 flex-col gap-8 md:grid', className)}>
      <div className="col-span-3">
        <Heading>{title}</Heading>
        <Text className={'mt-1'}>{description}</Text>
      </div>
      <div className="col-span-4">{children}</div>
    </div>
  </>
)

export default HeaderedContent
