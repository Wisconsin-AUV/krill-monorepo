export function randomInt(low: number, high: number): number {
  return Math.floor(Math.random() * (high - low) + low)
}
