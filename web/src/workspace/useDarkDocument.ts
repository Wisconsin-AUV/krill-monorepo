import { useEffect } from 'react'

// Dialogs portal to <body>, outside the workspace subtree, so dark mode has
// to be forced on the document for them to match.
export function useDarkDocument() {
  useEffect(() => {
    const root = document.documentElement
    root.classList.add('dark')
    return () => root.classList.remove('dark')
  }, [])
}
