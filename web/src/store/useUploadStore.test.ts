import { beforeEach, describe, expect, it, vi } from 'vitest'
import { useUploadStore, type UploadLink, type UploadTask } from './useUploadStore'

interface Held {
  task: UploadTask
  started: boolean
  finish: () => void
  fail: (err: Error) => void
  signal?: AbortSignal
}

function held(name: string): Held {
  const h = {} as Held
  h.started = false
  h.task = {
    name,
    size: 10,
    run: (signal) =>
      new Promise<UploadLink>((resolve, reject) => {
        h.started = true
        h.signal = signal
        h.finish = () => resolve({ to: `/${name}`, label: 'View' })
        h.fail = reject
        signal.addEventListener('abort', () => reject(new Error('aborted')))
      }),
  }
  return h
}

const state = (name: string) =>
  useUploadStore.getState().uploads.find((u) => u.name === name)?.state

beforeEach(() => useUploadStore.setState({ uploads: [] }))

describe('useUploadStore', () => {
  it('runs a few uploads at a time, starting the next as each one ends', async () => {
    const uploads = ['a', 'b', 'c', 'd', 'e'].map(held)
    useUploadStore.getState().start(uploads.map((u) => u.task))

    expect(uploads.map((u) => u.started)).toEqual([true, true, true, false, false])
    expect(state('d')).toBe('waiting')

    uploads[1].finish()
    await vi.waitFor(() => expect(uploads[3].started).toBe(true))
    expect(state('b')).toBe('done')
    expect(uploads[4].started).toBe(false)

    uploads[0].fail(new Error('offline'))
    await vi.waitFor(() => expect(uploads[4].started).toBe(true))
    expect(state('a')).toBe('error')
  })

  it('never starts a waiting upload that was canceled', async () => {
    const uploads = ['a', 'b', 'c', 'd'].map(held)
    useUploadStore.getState().start(uploads.map((u) => u.task))
    const waiting = useUploadStore.getState().uploads[3]

    useUploadStore.getState().cancel(waiting.id)
    expect(state('d')).toBe('canceled')
    uploads[0].finish()
    await vi.waitFor(() => expect(state('a')).toBe('done'))
    expect(uploads[3].started).toBe(false)
  })

  it('aborts a running upload when it is canceled', async () => {
    const upload = held('a')
    useUploadStore.getState().start([upload.task])

    useUploadStore.getState().cancel(useUploadStore.getState().uploads[0].id)
    expect(upload.signal?.aborted).toBe(true)
    await vi.waitFor(() => expect(state('a')).toBe('canceled'))
  })
})
