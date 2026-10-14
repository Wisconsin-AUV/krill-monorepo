import type { CorrectionRequest, CorrectionResponse } from './correction.worker'

const MAX_IMAGES = 24

interface Entry {
  ready: Promise<CanvasImageSource>
  image?: CanvasImageSource
}

const cache = new Map<bigint, Entry>()
const waiting = new Map<number, (r: CorrectionResponse) => void>()
let worker: Worker | undefined
let nextJob = 0

function run(bitmap: ImageBitmap): Promise<ImageBitmap> {
  if (!worker) {
    worker = new Worker(new URL('./correction.worker.ts', import.meta.url), { type: 'module' })
    worker.onmessage = (e: MessageEvent<CorrectionResponse>) => {
      waiting.get(e.data.job)?.(e.data)
      waiting.delete(e.data.job)
    }
  }
  const job = nextJob++
  const w = worker
  return new Promise((resolve, reject) => {
    waiting.set(job, (r) => ('bitmap' in r ? resolve(r.bitmap) : reject(new Error(r.error))))
    w.postMessage({ job, bitmap } satisfies CorrectionRequest, [bitmap])
  })
}

export function correctFrame(id: bigint, source: Promise<HTMLImageElement>): Entry {
  const hit = cache.get(id)
  if (hit) {
    cache.delete(id)
    cache.set(id, hit)
    return hit
  }
  const entry: Entry = {
    ready: source
      .then((img) =>
        createImageBitmap(img)
          .then(run)
          // If the pixels cannot be read, showing the raw frame beats showing none.
          .catch((err: unknown): CanvasImageSource => {
            console.warn(`Display correction failed for frame ${id}`, err)
            return img
          }),
      )
      .then((image) => (entry.image = image)),
  }
  entry.ready.catch(() => cache.delete(id))
  cache.set(id, entry)
  while (cache.size > MAX_IMAGES) {
    const oldest = cache.keys().next().value
    if (oldest === undefined) break
    cache.delete(oldest)
  }
  return entry
}
