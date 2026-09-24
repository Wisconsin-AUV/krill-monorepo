// The password slot opens with the field hidden, then the field fades in.
// Anything else that appears with it takes its space at once and fades in
// at the same moment.
const OPEN_S = 0.067
const FADE_S = 0.117

export const revealPassword = {
  initial: { height: 0, marginTop: '-2rem', opacity: 0, overflow: 'hidden' },
  animate: {
    height: 'auto',
    marginTop: 0,
    opacity: 1,
    transitionEnd: { overflow: 'visible' },
    transition: {
      height: { duration: OPEN_S, ease: 'easeOut' },
      marginTop: { duration: OPEN_S, ease: 'easeOut' },
      opacity: { delay: OPEN_S, duration: FADE_S, ease: 'linear' },
    },
  },
  exit: { opacity: 0, transition: { duration: 0 } },
} as const

export const fadeWithPassword = {
  initial: { opacity: 0 },
  animate: { opacity: 1, transition: { delay: OPEN_S, duration: FADE_S, ease: 'linear' } },
  exit: { opacity: 0, transition: { duration: 0 } },
} as const
