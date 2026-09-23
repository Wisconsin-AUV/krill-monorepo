import type React from 'react'
import { Subheading } from '@/components/ui/Heading'
import { Text } from '@/components/ui/Text'

export function EmptyState({
  icon: Icon,
  title,
  description,
  children,
}: {
  icon: React.ComponentType<React.SVGProps<SVGSVGElement>>
  title: string
  description: string
  children?: React.ReactNode
}) {
  return (
    <div className="flex flex-col items-center rounded-xl border border-dashed border-zinc-950/15 px-6 py-16 text-center dark:border-white/15">
      <Icon className="size-10 text-zinc-400 dark:text-zinc-500" aria-hidden="true" />
      <Subheading className="mt-4">{title}</Subheading>
      <Text className="mt-1 max-w-sm">{description}</Text>
      {children && <div className="mt-6">{children}</div>}
    </div>
  )
}
