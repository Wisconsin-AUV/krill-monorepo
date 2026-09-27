import { Dialog, DialogBody, DialogTitle } from '@/components/ui/Dialog'
import type { ShortcutGroup } from './shortcuts'

export function Kbd({ children }: { children: React.ReactNode }) {
  return (
    <kbd className="inline-flex min-w-6 items-center justify-center rounded border border-zinc-950/10 bg-zinc-950/[2.5%] px-1.5 font-sans text-xs/5 font-medium text-zinc-700 dark:border-white/15 dark:bg-white/5 dark:text-zinc-200">
      {children}
    </kbd>
  )
}

export function ShortcutsDialog({
  open,
  onClose,
  groups,
}: {
  open: boolean
  onClose: () => void
  groups: ShortcutGroup[]
}) {
  return (
    <Dialog open={open} onClose={onClose} size="2xl">
      <DialogTitle>Keyboard shortcuts</DialogTitle>
      <DialogBody className="grid gap-x-10 gap-y-8 sm:grid-cols-2">
        {groups.map((g) => (
          <section key={g.title}>
            <h3 className="text-xs/6 font-medium text-zinc-500 dark:text-zinc-400">{g.title}</h3>
            <dl className="mt-2 space-y-2">
              {g.items.map((s) => (
                <div key={s.label} className="flex items-center justify-between gap-4 text-sm/6">
                  <dt className="text-zinc-700 dark:text-zinc-300">{s.label}</dt>
                  <dd className="flex shrink-0 gap-1">
                    {s.keys.map((k) => (
                      <Kbd key={k}>{k}</Kbd>
                    ))}
                  </dd>
                </div>
              ))}
            </dl>
          </section>
        ))}
      </DialogBody>
    </Dialog>
  )
}
