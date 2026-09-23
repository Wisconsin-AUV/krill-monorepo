export function formatDuration(ms: bigint | number): string {
  const total = Math.max(0, Math.round(Number(ms) / 1000))
  const h = Math.floor(total / 3600)
  const m = Math.floor((total % 3600) / 60)
  const s = total % 60
  const ss = String(s).padStart(2, '0')
  return h > 0 ? `${h}:${String(m).padStart(2, '0')}:${ss}` : `${m}:${ss}`
}

export function formatFrameTime(ms: bigint | number): string {
  const n = Number(ms)
  return `${formatDuration(Math.floor(n / 1000) * 1000)}.${Math.floor((n % 1000) / 100)}`
}

export function formatNumber(n: bigint | number): string {
  return Number(n).toLocaleString()
}

export function plural(n: bigint | number, word: string, pluralWord = `${word}s`): string {
  return `${formatNumber(n)} ${Number(n) === 1 ? word : pluralWord}`
}

export function formatBytes(bytes: number): string {
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let v = bytes
  let i = 0
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024
    i++
  }
  return `${v.toFixed(i === 0 || v >= 100 ? 0 : 1)} ${units[i]}`
}

export function formatFps(fps: number): string {
  return `${Number.isInteger(fps) ? fps : fps.toFixed(2)} fps`
}

const relative = new Intl.RelativeTimeFormat(undefined, { numeric: 'auto' })
const steps: [Intl.RelativeTimeFormatUnit, number][] = [
  ['year', 365 * 24 * 3600],
  ['month', 30 * 24 * 3600],
  ['week', 7 * 24 * 3600],
  ['day', 24 * 3600],
  ['hour', 3600],
  ['minute', 60],
]

export function formatRelative(date: Date, now = new Date()): string {
  const seconds = (date.getTime() - now.getTime()) / 1000
  for (const [unit, size] of steps) {
    if (Math.abs(seconds) >= size) return relative.format(Math.round(seconds / size), unit)
  }
  return 'just now'
}
