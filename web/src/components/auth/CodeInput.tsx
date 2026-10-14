import { Input } from '@/components/ui/Input'

export const CODE_LENGTH = 6

export function CodeInput({
  value,
  onChange,
}: {
  value: string
  onChange: (value: string) => void
}) {
  return (
    <Input
      name="code"
      value={value}
      onChange={(e) => onChange(e.target.value.replace(/\D/g, '').slice(0, CODE_LENGTH))}
      inputMode="numeric"
      autoComplete="one-time-code"
      required
      className="tabular-nums"
    />
  )
}
