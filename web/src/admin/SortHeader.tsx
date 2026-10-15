import { ChevronDownIcon, ChevronUpIcon } from '@heroicons/react/16/solid'
import { clsx } from 'clsx'
import { TableHeader } from '@/components/ui/Table'
import type { Sort } from './useSort'

export function SortHeader<K extends string>({
  label,
  column,
  sort,
  onSort,
  className,
}: {
  label: string
  column: K
  sort: Sort<K>
  onSort: (key: K) => void
  className?: string
}) {
  const active = sort.key === column
  const Icon = active && !sort.desc ? ChevronUpIcon : ChevronDownIcon
  return (
    <TableHeader
      className={className}
      aria-sort={active ? (sort.desc ? 'descending' : 'ascending') : undefined}
    >
      <button
        type="button"
        onClick={() => onSort(column)}
        className="group inline-flex items-center gap-1 hover:text-zinc-950 dark:hover:text-white"
      >
        {label}
        <Icon
          className={clsx('size-4', active ? 'opacity-100' : 'opacity-0 group-hover:opacity-50')}
        />
      </button>
    </TableHeader>
  )
}
