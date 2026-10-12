import { correctPixels } from './correction'

export interface CorrectionRequest {
  job: number
  bitmap: ImageBitmap
}

export type CorrectionResponse =
  { job: number; bitmap: ImageBitmap } | { job: number; error: string }

self.onmessage = (e: MessageEvent<CorrectionRequest>) => {
  const { job, bitmap } = e.data
  try {
    const canvas = new OffscreenCanvas(bitmap.width, bitmap.height)
    const ctx = canvas.getContext('2d', { willReadFrequently: true })
    if (!ctx) throw new Error('no 2d context')
    ctx.drawImage(bitmap, 0, 0)
    bitmap.close()
    const pixels = ctx.getImageData(0, 0, canvas.width, canvas.height)
    correctPixels(pixels.data, pixels.width, pixels.height)
    ctx.putImageData(pixels, 0, 0)
    const out = canvas.transferToImageBitmap()
    self.postMessage({ job, bitmap: out } satisfies CorrectionResponse, { transfer: [out] })
  } catch (err) {
    self.postMessage({ job, error: String(err) } satisfies CorrectionResponse)
  }
}
