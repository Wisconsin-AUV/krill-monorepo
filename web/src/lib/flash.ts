import { useUIStore } from '@/store/useUIStore'
import { errorMessage } from './errors'

export const flash = {
  success: (title: string, message = '') =>
    useUIStore.getState().addFlash({ key: 'global', type: 'success', title, message }),
  error: (title: string, err: unknown) =>
    useUIStore
      .getState()
      .addFlash({ key: 'global', type: 'error', title, message: errorMessage(err) }, 8000),
}
