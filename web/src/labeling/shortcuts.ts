import type { ShortcutGroup } from '@/workspace/shortcuts'

export const labelingShortcuts: ShortcutGroup = {
  title: 'Labeling',
  items: [
    { keys: ['1–9'], label: 'Pick label type' },
    { keys: ['Drag'], label: 'Draw a box (continues the selected track)' },
    { keys: ['Tab'], label: 'Select next box on frame' },
    { keys: ['Esc'], label: 'Deselect (next box starts a new track)' },
    { keys: ['C'], label: 'Copy boxes from previous frame' },
    { keys: ['Del'], label: 'Delete selected box' },
    { keys: ['Shift', 'Del'], label: 'Delete selected track' },
    { keys: ['H'], label: 'Hide or show boxes' },
    { keys: ['⌘', 'Z'], label: 'Undo' },
  ],
}

export const frameShortcuts: ShortcutGroup = {
  title: 'Frames',
  items: [
    { keys: ['Space'], label: 'Mark labeled and go to next frame' },
    { keys: ['E'], label: 'Mark empty and go to next frame' },
    { keys: ['U'], label: 'Mark unlabeled' },
  ],
}
