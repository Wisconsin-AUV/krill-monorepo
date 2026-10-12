const TILES = 8
const CLIP_LIMIT = 2.5
// Murky water can leave the red channel nearly empty, and an unbounded
// gray-world gain would turn its sensor noise into red speckle.
const MAX_GAIN = 3

function clampByte(v: number): number {
  return v < 0 ? 0 : v > 255 ? 255 : v
}

function grayWorldLut(data: Uint8ClampedArray): Uint8Array[] {
  let r = 0
  let g = 0
  let b = 0
  for (let i = 0; i < data.length; i += 4) {
    r += data[i]
    g += data[i + 1]
    b += data[i + 2]
  }
  const gray = (r + g + b) / 3
  return [r, g, b].map((sum) => {
    const gain = sum > 0 ? Math.min(gray / sum, MAX_GAIN) : 1
    const lut = new Uint8Array(256)
    for (let v = 0; v < 256; v++) lut[v] = clampByte(Math.round(v * gain))
    return lut
  })
}

function tileLut(hist: Uint32Array, count: number, out: Uint8Array) {
  const limit = Math.max(1, Math.floor((CLIP_LIMIT * count) / 256))
  let excess = 0
  for (let v = 0; v < 256; v++) {
    if (hist[v] > limit) {
      excess += hist[v] - limit
      hist[v] = limit
    }
  }
  const spread = excess / 256
  let cdf = 0
  for (let v = 0; v < 256; v++) {
    cdf += hist[v] + spread
    out[v] = clampByte(Math.round((cdf * 255) / count))
  }
}

// For each pixel along one axis: the two tiles whose centres surround it and
// the weight of the second, so tile seams are interpolated away.
function axisWeights(size: number, tiles: number, tileSize: number) {
  const lo = new Uint8Array(size)
  const hi = new Uint8Array(size)
  const w = new Float32Array(size)
  for (let p = 0; p < size; p++) {
    const f = (p + 0.5) / tileSize - 0.5
    const t = Math.floor(f)
    lo[p] = Math.min(Math.max(t, 0), tiles - 1)
    hi[p] = Math.min(Math.max(t + 1, 0), tiles - 1)
    w[p] = f < 0 ? 0 : f - t
  }
  return { lo, hi, w }
}

// Gray-world white balance, then CLAHE on luma, in place on RGBA pixels. The
// luma change is added to every channel equally so hue is kept.
export function correctPixels(data: Uint8ClampedArray, width: number, height: number) {
  if (width === 0 || height === 0) return
  const [lr, lg, lb] = grayWorldLut(data)
  const n = width * height
  const luma = new Uint8Array(n)
  for (let p = 0, i = 0; p < n; p++, i += 4) {
    const r = lr[data[i]]
    const g = lg[data[i + 1]]
    const b = lb[data[i + 2]]
    data[i] = r
    data[i + 1] = g
    data[i + 2] = b
    luma[p] = (77 * r + 150 * g + 29 * b) >> 8
  }

  const tx = Math.min(TILES, width)
  const ty = Math.min(TILES, height)
  const tileW = Math.ceil(width / tx)
  const tileH = Math.ceil(height / ty)
  const hists = new Uint32Array(tx * ty * 256)
  const counts = new Uint32Array(tx * ty)
  for (let y = 0; y < height; y++) {
    const row = Math.min(Math.floor(y / tileH), ty - 1) * tx
    for (let x = 0; x < width; x++) {
      const t = row + Math.min(Math.floor(x / tileW), tx - 1)
      hists[t * 256 + luma[y * width + x]]++
      counts[t]++
    }
  }
  const luts = new Uint8Array(tx * ty * 256)
  for (let t = 0; t < tx * ty; t++) {
    if (counts[t] === 0) continue
    const at = t * 256
    tileLut(hists.subarray(at, at + 256), counts[t], luts.subarray(at, at + 256))
  }

  const cols = axisWeights(width, tx, tileW)
  const rows = axisWeights(height, ty, tileH)
  for (let y = 0; y < height; y++) {
    const top = rows.lo[y] * tx
    const bottom = rows.hi[y] * tx
    const wy = rows.w[y]
    for (let x = 0; x < width; x++) {
      const p = y * width + x
      const v = luma[p]
      const l = cols.lo[x]
      const r = cols.hi[x]
      const wx = cols.w[x]
      const a = luts[(top + l) * 256 + v] * (1 - wx) + luts[(top + r) * 256 + v] * wx
      const b = luts[(bottom + l) * 256 + v] * (1 - wx) + luts[(bottom + r) * 256 + v] * wx
      const delta = Math.round(a * (1 - wy) + b * wy) - v
      const i = p * 4
      data[i] += delta
      data[i + 1] += delta
      data[i + 2] += delta
    }
  }
}
