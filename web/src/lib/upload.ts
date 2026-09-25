export function putFile(
  url: string,
  file: Blob,
  onProgress: (loaded: number) => void,
  signal: AbortSignal,
): Promise<void> {
  // fetch() has no upload progress events, so this uses XHR.
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest()
    xhr.open('PUT', url)
    xhr.upload.onprogress = (e) => onProgress(e.loaded)
    xhr.onload = () =>
      xhr.status >= 200 && xhr.status < 300
        ? resolve()
        : reject(new Error(`Upload failed with HTTP ${xhr.status}`))
    xhr.onerror = () => reject(new Error('Upload failed. Check that MinIO is reachable.'))
    xhr.onabort = () => reject(new DOMException('Upload canceled', 'AbortError'))
    signal.addEventListener('abort', () => xhr.abort(), { once: true })
    xhr.send(file)
  })
}

const partConcurrency = 3

export async function putParts(
  urls: string[],
  partSize: number,
  file: File,
  onProgress: (loaded: number) => void,
  signal: AbortSignal,
): Promise<void> {
  const failed = new AbortController()
  const stop = AbortSignal.any([signal, failed.signal])
  const loaded = new Array<number>(urls.length).fill(0)
  let next = 0
  async function worker() {
    while (next < urls.length && !stop.aborted) {
      const i = next++
      const part = file.slice(i * partSize, (i + 1) * partSize)
      await putFile(
        urls[i],
        part,
        (n) => {
          loaded[i] = n
          onProgress(loaded.reduce((a, b) => a + b, 0))
        },
        stop,
      ).catch((err: unknown) => {
        failed.abort()
        throw err
      })
    }
  }
  await Promise.all(Array.from({ length: Math.min(partConcurrency, urls.length) }, worker))
}
