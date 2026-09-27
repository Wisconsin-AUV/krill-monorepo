const MAX_IMAGES = 80

interface Entry {
  img: HTMLImageElement
  ready: Promise<HTMLImageElement>
  loaded: boolean
}

// Keyed by frame id rather than URL: presigned URLs change on every fetch of
// the clip, but the image behind a frame id never does.
const cache = new Map<bigint, Entry>()

export function loadFrame(id: bigint, url: string): Entry {
  const hit = cache.get(id)
  if (hit) {
    cache.delete(id)
    cache.set(id, hit)
    return hit
  }
  const img = new Image()
  img.decoding = 'async'
  const entry: Entry = {
    img,
    loaded: false,
    ready: new Promise((resolve, reject) => {
      img.onload = () => {
        entry.loaded = true
        resolve(img)
      }
      img.onerror = () => {
        cache.delete(id)
        reject(new Error(`Failed to load frame ${id}`))
      }
    }),
  }
  entry.ready.catch(() => undefined)
  img.src = url
  cache.set(id, entry)
  while (cache.size > MAX_IMAGES) {
    const oldest = cache.keys().next().value
    if (oldest === undefined) break
    cache.delete(oldest)
  }
  return entry
}

export function isFrameLoaded(id: bigint): boolean {
  return cache.get(id)?.loaded ?? false
}
