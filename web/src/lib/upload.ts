export function putFile(
  url: string,
  file: File,
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
