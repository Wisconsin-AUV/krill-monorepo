import { useEffect, useRef } from 'react'

export type HotkeyMap = Record<string, (e: KeyboardEvent) => void>

function keyName(e: KeyboardEvent): string {
  const parts = []
  if (e.metaKey || e.ctrlKey) parts.push('mod')
  if (e.shiftKey && e.key.length > 1) parts.push('shift')
  if (e.altKey) parts.push('alt')
  parts.push(e.key.length === 1 ? e.key.toLowerCase() : e.key)
  return parts.join('+')
}

function isTyping(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false
  return target.isContentEditable || ['INPUT', 'TEXTAREA', 'SELECT'].includes(target.tagName)
}

// Keys are matched as e.g. "k", "K" (shift), "shift+ArrowRight", "mod+z".
export function useHotkeys(map: HotkeyMap) {
  const ref = useRef(map)
  useEffect(() => {
    ref.current = map
  })

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (isTyping(e.target) || document.querySelector('[role=dialog], [role=alertdialog]')) return
      const plainChar = e.key.length === 1 && !e.metaKey && !e.ctrlKey && !e.altKey
      const handler = (plainChar && ref.current[e.key]) || ref.current[keyName(e)]
      if (handler) {
        e.preventDefault()
        handler(e)
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [])
}
