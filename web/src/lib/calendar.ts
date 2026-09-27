export interface CalendarDay {
  date: Date
  key: string
  count: number
}

export function dateKey(d: Date): string {
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${d.getFullYear()}-${m}-${day}`
}

// Weeks run Monday to Sunday, matching the API's contribution window: the
// first week starts 52 weeks before the current one. Days after today are
// left out of the last week.
export function buildCalendar(counts: Map<string, number>, today = new Date()): CalendarDay[][] {
  const start = new Date(today.getFullYear(), today.getMonth(), today.getDate())
  start.setDate(start.getDate() - ((start.getDay() + 6) % 7) - 52 * 7)
  const todayKey = dateKey(today)

  const weeks: CalendarDay[][] = []
  const d = new Date(start)
  for (let w = 0; w < 53; w++) {
    const week: CalendarDay[] = []
    for (let i = 0; i < 7; i++) {
      const key = dateKey(d)
      week.push({ date: new Date(d), key, count: counts.get(key) ?? 0 })
      if (key === todayKey) return [...weeks, week]
      d.setDate(d.getDate() + 1)
    }
    weeks.push(week)
  }
  return weeks
}

export function level(count: number, max: number): number {
  if (count === 0 || max === 0) return 0
  return Math.min(4, Math.ceil((count / max) * 4))
}
