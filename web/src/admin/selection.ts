import { useState } from 'react'

// Selected row keys. Keys that leave the list stay out of `selected`, so a
// refetch never acts on rows that are gone.
export function useSelection(keys: string[]) {
  const [picked, setPicked] = useState<ReadonlySet<string>>(new Set())
  const selected = keys.filter((k) => picked.has(k))
  const all = keys.length > 0 && selected.length === keys.length
  return {
    selected,
    has: (key: string) => picked.has(key),
    toggle: (key: string, on: boolean) =>
      setPicked((s) => {
        const next = new Set(s)
        if (on) next.add(key)
        else next.delete(key)
        return next
      }),
    all,
    some: selected.length > 0 && !all,
    toggleAll: () => setPicked(all ? new Set() : new Set(keys)),
    clear: () => setPicked(new Set()),
  }
}
