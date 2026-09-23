export interface Shortcut {
  keys: string[]
  label: string
}

export interface ShortcutGroup {
  title: string
  items: Shortcut[]
}

export const navigationShortcuts: ShortcutGroup = {
  title: 'Navigation',
  items: [
    { keys: ['J', '←'], label: 'Previous frame' },
    { keys: ['K', '→'], label: 'Next frame' },
    { keys: ['Shift', 'J / K'], label: 'Jump 10 frames' },
    { keys: ['Home', 'End'], label: 'First or last frame' },
    { keys: ['P'], label: 'Play or pause' },
    { keys: ['[', ']'], label: 'Previous or next clip' },
  ],
}

export const viewShortcuts: ShortcutGroup = {
  title: 'View',
  items: [
    { keys: ['Scroll'], label: 'Zoom at cursor' },
    { keys: ['Right drag'], label: 'Pan (or Alt + drag)' },
    { keys: ['F'], label: 'Fit frame to screen' },
    { keys: ['?'], label: 'Show shortcuts' },
  ],
}
