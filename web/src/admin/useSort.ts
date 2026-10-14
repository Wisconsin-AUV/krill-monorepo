import { useMemo, useState } from 'react'

export type SortValue = number | string | undefined

export type Sort<K extends string> = { key: K; desc: boolean }

// Rows without a value sort last whichever way the column is ordered.
export function useSort<T, K extends string>(
  rows: T[],
  value: (row: T, key: K) => SortValue,
  initial: Sort<K>,
) {
  const [sort, setSort] = useState(initial)
  const sorted = useMemo(() => {
    const out = [...rows]
    out.sort((a, b) => {
      const x = value(a, sort.key)
      const y = value(b, sort.key)
      if (x === undefined || y === undefined) return x === y ? 0 : x === undefined ? 1 : -1
      const c = typeof x === 'string' ? x.localeCompare(String(y)) : x - (y as number)
      return sort.desc ? -c : c
    })
    return out
  }, [rows, value, sort])
  const toggle = (key: K) =>
    setSort((s) => (s.key === key ? { key, desc: !s.desc } : { key, desc: true }))
  return { sorted, sort, toggle }
}
