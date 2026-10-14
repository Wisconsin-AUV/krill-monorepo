import { Alert, AlertActions, AlertDescription, AlertTitle } from '@/components/ui/Alert'
import { Button } from '@/components/ui/Button'
import { FormError } from '@/components/ui/Callout'

export function ConfirmAlert({
  open,
  onClose,
  onConfirm,
  title,
  description,
  confirmLabel,
  busy = false,
  error,
}: {
  open: boolean
  onClose: () => void
  onConfirm: () => void
  title: string
  description: string
  confirmLabel: string
  busy?: boolean
  error?: unknown
}) {
  return (
    <Alert open={open} onClose={onClose}>
      <AlertTitle>{title}</AlertTitle>
      <AlertDescription>{description}</AlertDescription>
      <FormError error={error} className="mt-4" />
      <AlertActions>
        <Button plain onClick={onClose} disabled={busy}>
          Cancel
        </Button>
        <Button color="red" onClick={onConfirm} disabled={busy}>
          {confirmLabel}
        </Button>
      </AlertActions>
    </Alert>
  )
}
