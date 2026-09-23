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
  description: string
  attributes: { name: string; options: string[] }[]
}

// From the task table in docs/design-doc.md. Roles are left out because the
// season's role list is still an open question.
export const starterTypes: StarterType[] = [
  { name: 'gate', description: 'The gate frame.', attributes: [] },
  { name: 'gate_marker', description: 'Role markers on the gate.', attributes: [] },
  {
    name: 'torpedo_hole',
    description: 'Torpedo target holes.',
    attributes: [{ name: 'size', options: ['big', 'small'] }],
  },
  { name: 'bin', description: 'A bin. Assign its role from the image inside.', attributes: [] },
  { name: 'octagon_image', description: 'A vinyl image inside the octagon.', attributes: [] },
  { name: 'table_item', description: 'An item on the table.', attributes: [] },
  { name: 'basket', description: 'The basket on the table.', attributes: [] },
  { name: 'path_marker', description: 'Path marker on the pool floor.', attributes: [] },
  {
    name: 'slalom_pipe',
    description: 'A slalom pipe.',
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
