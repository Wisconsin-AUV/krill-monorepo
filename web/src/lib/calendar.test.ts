import { describe, expect, it, vi } from 'vitest'
import { buildCalendar } from './calendar'

describe('buildCalendar', () => {
  it('spans 53 Monday-first weeks ending today, across a DST change', () => {
    vi.stubEnv('TZ', 'America/Chicago')
    const today = new Date(2025, 10, 12, 15)
    const weeks = buildCalendar(new Map([['2025-11-02', 3]]), today)

    expect(weeks).toHaveLength(53)
    expect(weeks[0][0].key).toBe('2024-11-11')
    expect(weeks[0][0].date.getDay()).toBe(1)
    expect(weeks.at(-1)?.at(-1)?.key).toBe('2025-11-12')
    const days = weeks.flat()
    expect(new Set(days.map((d) => d.key)).size).toBe(days.length)
    expect(days.find((d) => d.key === '2025-11-02')?.count).toBe(3)
  })
})
