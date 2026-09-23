import { Button } from '@/components/ui/Button'

export function FormError({ message }: { message: string | null }) {
  if (!message) return null
  return (
    <p role="alert" className="mt-6 text-sm/6 text-red-600 dark:text-red-400">
      {message}
    </p>
  )
}

export function SlackButton({ label }: { label: string }) {
  return (
    <>
      <div className="my-6 flex items-center gap-3 text-xs/6 text-zinc-500 dark:text-zinc-400">
        <hr className="flex-1 border-zinc-950/10 dark:border-white/10" />
        or
        <hr className="flex-1 border-zinc-950/10 dark:border-white/10" />
      </div>
      <Button outline to="/auth/slack/login" reloadDocument className="w-full">
        {label}
      </Button>
    </>
  )
}
