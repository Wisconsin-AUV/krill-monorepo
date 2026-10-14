import { describe, expect, it } from 'vitest'
import { correctPixels } from './correction'

function image(width: number, height: number, pixel: (x: number, y: number) => number[]) {
  const data = new Uint8ClampedArray(width * height * 4)
  for (let y = 0; y < height; y++) {
    for (let x = 0; x < width; x++) data.set([...pixel(x, y), 255], (y * width + x) * 4)
  }
  return data
}

describe('correctPixels', () => {
  it('keeps gray pixels gray', () => {
    const data = image(64, 48, (x, y) => {
      const v = (x * 3 + y * 2) % 256
      return [v, v, v]
    })
    correctPixels(data, 64, 48)
    for (let i = 0; i < data.length; i += 4) {
      expect(data[i]).toBe(data[i + 1])
      expect(data[i]).toBe(data[i + 2])
      expect(data[i + 3]).toBe(255)
    }
  })

  it('balances a green-blue cast toward neutral', () => {
    const data = image(32, 32, (x) => [20 + (x % 8), 90 + (x % 8), 110 + (x % 8)])
    correctPixels(data, 32, 32)
    const mean = [0, 0, 0]
    for (let i = 0; i < data.length; i += 4) for (let c = 0; c < 3; c++) mean[c] += data[i + c]
    const spread = Math.max(...mean) - Math.min(...mean)
    expect(spread / (32 * 32)).toBeLessThan(15)
  })

  it('stretches low-contrast luma', () => {
    const data = image(80, 60, (x, y) => {
      const v = 100 + ((x + y) % 20)
      return [v, v, v]
    })
    correctPixels(data, 80, 60)
    let min = 255
    let max = 0
    for (let i = 0; i < data.length; i += 4) {
      min = Math.min(min, data[i])
      max = Math.max(max, data[i])
    }
    expect(max - min).toBeGreaterThan(40)
  })
})
