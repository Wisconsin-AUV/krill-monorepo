import { useAnimate, useReducedMotion } from 'motion/react'
import { useEffect, useRef, useState } from 'react'

const SHAKE_MS = 400

// Shakes the scoped element each time shake changes. shaking stays true until
// the latest shake has played out, for keeping the field red meanwhile.
export function useShake(shake: number) {
  const [settledShake, setSettledShake] = useState(shake)
  const [scope, animate] = useAnimate<HTMLDivElement>()
  const reduceMotion = useReducedMotion()

  // Only a change after mount means a new failure, so a field that remounts
  // with an old count stays still.
  const lastShake = useRef(shake)
  useEffect(() => {
    if (shake === lastShake.current) return
    lastShake.current = shake
    if (!reduceMotion) {
      void animate(scope.current, { x: [0, 5, -4, 4, -2, 0] }, { duration: SHAKE_MS / 1000 })
    }
    const done = setTimeout(() => setSettledShake(shake), SHAKE_MS)
    return () => clearTimeout(done)
  }, [shake, reduceMotion, animate, scope])

  return { scope, shaking: shake !== settledShake }
}
