import { useUIStore } from '@/store/useUIStore'
import { errorMessage } from './errors'

const add = useUIStore.getState().addFlash

export const flash = {
  success: (title: string, message = '') => add({ type: 'success', title, message }),
  undoable: (title: string, undo: () => void) =>
    add({ type: 'success', title, message: '', action: { label: 'Undo', onClick: undo } }, 8000),
  info: (title: string, message = '') => add({ type: 'info', title, message }),
  error: (title: string, err: unknown) =>
    add({ type: 'error', title, message: errorMessage(err) }, 8000),
}
