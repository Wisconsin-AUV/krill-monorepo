import type { ShortcutGroup } from '@/workspace/shortcuts'

export const labelingShortcuts: ShortcutGroup = {
  title: 'Labeling',
  items: [
    { keys: ['1–9'], label: 'Pick label type' },
    { keys: ['Drag'], label: 'Draw a box (continues the selected track)' },
    { keys: ['Shift', 'Click'], label: 'Track a new object through the clip' },
    { keys: ['Shift', 'Drag'], label: 'Draw a box and track it through the clip' },
    { keys: ['R'], label: 'Track the selected box from this frame on' },
    { keys: ['Shift', 'X'], label: 'Find objects like the selected box and track them' },
    { keys: ['Click'], label: 'Place the keypoint named by the cursor' },
    { keys: ['Alt', 'Click'], label: 'Place it hidden' },
    { keys: ['Shift', 'Click'], label: 'Fill a quad with SAM, inside the selected box' },
    { keys: ['O'], label: 'Keypoint visible or hidden' },
    { keys: ['X'], label: 'Clear keypoint' },
    { keys: ['1–9'], label: 'Pick which keypoint to place, on a box with keypoints' },
    { keys: ['W', 'Q'], label: 'Next or previous keypoint' },
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
    { keys: ['Space'], label: 'Mark labeled, accept tracked boxes, and go to next frame' },
    { keys: ['E'], label: 'Mark empty and go to next frame' },
    { keys: ['U'], label: 'Mark unlabeled' },
    { keys: ['D'], label: 'Next frame with a drifted box' },
    { keys: ['M'], label: 'Flag frame for review or unflag it' },
  ],
}

export const goldShortcut = { keys: ['G'], label: 'Make frame gold or remove it' }

export const trialShortcuts: ShortcutGroup = {
  title: 'Model trials',
  items: [
    { keys: ['B'], label: 'Next frame where the model disagrees' },
    { keys: ['Shift', 'B'], label: 'Hide or show model boxes' },
  ],
}
