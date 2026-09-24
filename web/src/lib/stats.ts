import { Period } from '@/gen/krill/v1/stats_pb'

export const timeZone = Intl.DateTimeFormat().resolvedOptions().timeZone

export const periods = [
  { period: Period.DAY, key: 'day', label: 'Today' },
  { period: Period.WEEK, key: 'week', label: 'This week' },
  { period: Period.MONTH, key: 'month', label: 'This month' },
  { period: Period.ALL_TIME, key: 'all', label: 'All time' },
] as const

export function periodFromKey(key: string | null) {
  return periods.find((p) => p.key === key) ?? periods[1]
}

export function ordinal(n: number): string {
  const rem100 = n % 100
  if (rem100 >= 11 && rem100 <= 13) return `${n}th`
  return `${n}${['th', 'st', 'nd', 'rd'][n % 10] ?? 'th'}`
}
