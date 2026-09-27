import type { LabelAttribute, LabelType } from '@/gen/krill/v1/label_pb'

export const palette = [
  '#0ea5e9',
  '#f97316',
  '#84cc16',
  '#f43f5e',
  '#8b5cf6',
  '#f59e0b',
  '#10b981',
  '#d946ef',
  '#06b6d4',
  '#ef4444',
  '#eab308',
  '#3b82f6',
]

export interface StarterType {
  name: string
  title: string
  description: string
  guideline: string
  attributes: { name: string; options: string[] }[]
}

// From docs/labeling-guideline.md. Roles are left out because the season's
// role list is still an open question.
export const starterTypes: StarterType[] = [
  {
    name: 'gate',
    title: 'Gate',
    description: 'The gate frame.',
    guideline: 'Box the entire frame.',
    attributes: [],
  },
  {
    name: 'role_sign',
    title: 'Role sign',
    description: 'A panel hanging from the gate, or an image on the octagon wall.',
    guideline: 'Box the entire panel or image.',
    attributes: [],
  },
  {
    name: 'torpedo_board',
    title: 'Torpedo board',
    description: 'The torpedo target board.',
    guideline: 'Box the outer edge of the board.',
    attributes: [],
  },
  {
    name: 'torpedo_hole',
    title: 'Torpedo hole',
    description: 'A hole in the torpedo board.',
    guideline: 'Box the hole opening tightly.',
    attributes: [{ name: 'size', options: ['big', 'small'] }],
  },
  {
    name: 'bin',
    title: 'Bin',
    description: 'A bin on the pool floor.',
    guideline: 'Box the whole bin opening, including the image.',
    attributes: [],
  },
  {
    name: 'table',
    title: 'Table',
    description: 'The table.',
    guideline: 'Box the tabletop.',
    attributes: [],
  },
  {
    name: 'table_item',
    title: 'Table item',
    description: 'An item on the table.',
    guideline: 'Box each object separately.',
    attributes: [],
  },
  {
    name: 'basket',
    title: 'Basket',
    description: 'The basket on the table.',
    guideline: 'Box the entire basket.',
    attributes: [],
  },
  {
    name: 'path_marker',
    title: 'Path marker',
    description: 'Path marker on the pool floor.',
    guideline: '',
    attributes: [],
  },
  {
    name: 'slalom_pipe',
    title: 'Slalom pipe',
    description: 'A slalom pipe.',
    guideline: '',
    attributes: [{ name: 'color', options: ['red', 'white'] }],
  },
]

export function exportClasses(
  name: string,
  attributes: Pick<LabelAttribute, 'name' | 'options'>[],
): string[] {
  let classes = [name]
  for (const a of attributes) {
    classes = classes.flatMap((c) => a.options.map((o) => `${c}-${o}`))
  }
  return classes
}

export function missingAttributes(
  type: LabelType | undefined,
  values: Record<string, string>,
): string[] {
  if (!type) return []
  return type.attributes.filter((a) => !a.options.includes(values[a.name])).map((a) => a.name)
}

export function withAlpha(hex: string, alpha: number): string {
  const a = Math.round(alpha * 255)
    .toString(16)
    .padStart(2, '0')
  return `${hex}${a}`
}
