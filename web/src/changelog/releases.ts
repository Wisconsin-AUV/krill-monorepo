import { useQuery } from '@tanstack/react-query'
import { parseChangelog } from './parse'

// The changelog ships with the build, so it describes the version running.
export function useReleases(enabled = true) {
  return useQuery({
    queryKey: ['changelog'],
    queryFn: () => import('virtual:changelog').then((m) => parseChangelog(m.default)),
    staleTime: Infinity,
    enabled,
  })
}
